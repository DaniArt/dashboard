package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/config"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
)

type ConfluenceCollector struct {
	cfg    *config.Config
	client *http.Client
}

func NewConfluenceCollector(cfg *config.Config) *ConfluenceCollector {
	return &ConfluenceCollector{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetEmployeeMetrics returns Confluence metrics for an employee
func (c *ConfluenceCollector) GetEmployeeMetrics(employee models.Employee) (models.ConfluenceMetrics, error) {
	var metrics models.ConfluenceMetrics

	// Confluence Server/DC uses username (without domain), not email
	username := extractConfUsername(employee.Email)

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Pages created this month
	createdMonth, err := c.searchPages(username, "created", monthStart)
	if err != nil {
		return metrics, err
	}
	metrics.PagesCreatedMonth = len(createdMonth)

	// Pages created today
	createdToday, err := c.searchPages(username, "created", todayStart)
	if err != nil {
		return metrics, err
	}
	metrics.PagesCreatedToday = len(createdToday)

	// Pages updated this month (contributor)
	updatedMonth, err := c.searchContributed(username, monthStart)
	if err != nil {
		return metrics, err
	}
	metrics.PagesUpdatedMonth = len(updatedMonth)

	// Total pages by this user
	total, err := c.getTotalPages(username)
	if err != nil {
		return metrics, err
	}
	metrics.TotalPages = total

	// Quality score calculation (simplified: based on activity)
	if metrics.PagesCreatedMonth+metrics.PagesUpdatedMonth > 10 {
		metrics.QualityScore = 90
	} else if metrics.PagesCreatedMonth+metrics.PagesUpdatedMonth > 5 {
		metrics.QualityScore = 70
	} else if metrics.PagesCreatedMonth+metrics.PagesUpdatedMonth > 0 {
		metrics.QualityScore = 50
	} else {
		metrics.QualityScore = 20
	}

	return metrics, nil
}

// GetEmployeePageDetails returns detailed page info for an employee (current month)
func (c *ConfluenceCollector) GetEmployeePageDetails(employee models.Employee) ([]models.ConfluencePage, error) {
	username := extractConfUsername(employee.Email)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	return c.searchPagesDetailedRange(username, monthStart, monthEnd)
}

// GetEmployeePageDetailsSince returns pages since a specific date (no upper bound)
func (c *ConfluenceCollector) GetEmployeePageDetailsSince(employee models.Employee, since time.Time) ([]models.ConfluencePage, error) {
	return c.GetEmployeePageDetailsRange(employee, since, time.Time{})
}

// GetEmployeePageDetailsRange returns pages in a date range
func (c *ConfluenceCollector) GetEmployeePageDetailsRange(employee models.Employee, since time.Time, until time.Time) ([]models.ConfluencePage, error) {
	username := extractConfUsername(employee.Email)
	return c.searchPagesDetailedRange(username, since, until)
}

func (c *ConfluenceCollector) searchPagesDetailedRange(username string, since time.Time, until time.Time) ([]models.ConfluencePage, error) {
	sinceStr := since.Format("2006-01-02")
	var cql string
	if until.IsZero() {
		cql = fmt.Sprintf(`(creator = "%s" OR contributor = "%s") AND lastModified >= "%s" AND type = page`, username, username, sinceStr)
	} else {
		untilStr := until.Format("2006-01-02")
		cql = fmt.Sprintf(`(creator = "%s" OR contributor = "%s") AND lastModified >= "%s" AND lastModified < "%s" AND type = page`, username, username, sinceStr, untilStr)
	}

	endpoint := fmt.Sprintf("%s/rest/api/content/search?cql=%s&limit=50&expand=version,space,body.storage",
		c.cfg.Confluence.URL, url.QueryEscape(cql))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Confluence.Token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence detail request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence returned %d: %s", resp.StatusCode, string(body))
	}

	var result confluenceDetailResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	now := time.Now()
	var pages []models.ConfluencePage
	for _, r := range result.Results {
		var lastUpdated time.Time
		if r.Version.When != "" {
			lastUpdated, _ = time.Parse("2006-01-02T15:04:05.000Z", r.Version.When)
			if lastUpdated.IsZero() {
				lastUpdated, _ = time.Parse(time.RFC3339, r.Version.When)
			}
		}

		bodyLen := 0
		if r.Body.Storage.Value != "" {
			bodyLen = len(r.Body.Storage.Value)
		}

		daysOld := 0
		if !lastUpdated.IsZero() {
			daysOld = int(now.Sub(lastUpdated).Hours() / 24)
		}

		pages = append(pages, models.ConfluencePage{
			ID:              r.ID,
			Title:           r.Title,
			Space:           r.Space.Key,
			SpaceName:       r.Space.Name,
			URL:             fmt.Sprintf("%s/pages/viewpage.action?pageId=%s", c.cfg.Confluence.URL, r.ID),
			Creator:         r.Version.By.DisplayName,
			LastUpdated:     lastUpdated,
			Version:         r.Version.Number,
			BodyLength:      bodyLen,
			DaysSinceUpdate: daysOld,
			Changes:         generateChangeDescription(r.Version.Number, bodyLen, r.Title),
		})
	}

	return pages, nil
}

func (c *ConfluenceCollector) searchPages(username, dateField string, since time.Time) ([]string, error) {
	sinceStr := since.Format("2006-01-02")
	cql := fmt.Sprintf(`creator = "%s" AND %s >= "%s" AND type = page`, username, dateField, sinceStr)

	endpoint := fmt.Sprintf("%s/rest/api/content/search?cql=%s&limit=100",
		c.cfg.Confluence.URL, url.QueryEscape(cql))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Confluence.Token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence returned %d: %s", resp.StatusCode, string(body))
	}

	var result confluenceSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var ids []string
	for _, r := range result.Results {
		ids = append(ids, r.ID)
	}
	return ids, nil
}

func (c *ConfluenceCollector) searchContributed(username string, since time.Time) ([]string, error) {
	sinceStr := since.Format("2006-01-02")
	cql := fmt.Sprintf(`contributor = "%s" AND lastModified >= "%s" AND type = page`, username, sinceStr)

	endpoint := fmt.Sprintf("%s/rest/api/content/search?cql=%s&limit=100",
		c.cfg.Confluence.URL, url.QueryEscape(cql))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Confluence.Token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("confluence request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback - some Confluence versions don't support contributor
		return nil, nil
	}

	var result confluenceSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var ids []string
	for _, r := range result.Results {
		ids = append(ids, r.ID)
	}
	return ids, nil
}

func (c *ConfluenceCollector) getTotalPages(username string) (int, error) {
	cql := fmt.Sprintf(`creator = "%s" AND type = page`, username)
	endpoint := fmt.Sprintf("%s/rest/api/content/search?cql=%s&limit=0",
		c.cfg.Confluence.URL, url.QueryEscape(cql))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Confluence.Token)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}

	var result confluenceSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.TotalSize, nil
}

type confluenceSearchResult struct {
	Results   []confluencePage `json:"results"`
	TotalSize int             `json:"totalSize"`
}

type confluencePage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type confluenceDetailResult struct {
	Results []confluenceDetailPage `json:"results"`
}

type confluenceDetailPage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Space   struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"space"`
	Version struct {
		Number int    `json:"number"`
		When   string `json:"when"`
		By     struct {
			DisplayName string `json:"displayName"`
		} `json:"by"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

// extractConfUsername extracts username from email (e.g., "DAltybassarova" from "DAltybassarova@Fortebank.com")
// Confluence Server/DC uses username, not email for CQL queries
func extractConfUsername(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

// generateChangeDescription creates a description based on version diff
func generateChangeDescription(version int, bodyLen int, title string) string {
	if version == 1 {
		if bodyLen < 200 {
			return "Создана страница (черновик)"
		}
		return "Создана новая страница"
	}
	if version == 2 {
		return "Первое обновление после создания"
	}
	if bodyLen < 500 {
		return fmt.Sprintf("Обновление v%d — минорные правки", version)
	} else if bodyLen < 2000 {
		return fmt.Sprintf("Обновление v%d — дополнено содержимое", version)
	} else if bodyLen < 10000 {
		return fmt.Sprintf("Обновление v%d — существенные изменения", version)
	}
	return fmt.Sprintf("Обновление v%d — обширная переработка (~%dKB)", version, bodyLen/1024)
}
