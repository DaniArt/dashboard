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

type GitLabCollector struct {
	cfg    *config.Config
	client *http.Client
}

func NewGitLabCollector(cfg *config.Config) *GitLabCollector {
	return &GitLabCollector{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetEmployeeMetrics returns GitLab metrics for an employee
func (g *GitLabCollector) GetEmployeeMetrics(employee models.Employee) (models.GitLabMetrics, error) {
	var metrics models.GitLabMetrics

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Get user's MRs
	mrs, err := g.getUserMRs(employee.Email, monthStart)
	if err != nil {
		return metrics, err
	}

	for _, mr := range mrs {
		metrics.MRsCreatedMonth++
		if mr.State == "merged" {
			metrics.MRsMergedMonth++
		}
		if mr.State == "opened" {
			metrics.MRsOpen++
			// Check if has reviewers
			if len(mr.Reviewers) == 0 {
				metrics.MRsWithoutReview++
			}
		}
	}

	// Get user's events/commits for the month
	commits, err := g.getUserCommits(employee, monthStart)
	if err != nil {
		// Non-critical, continue
		commits = 0
	}
	metrics.CommitsMonth = commits

	commitsToday, err := g.getUserCommits(employee, todayStart)
	if err != nil {
		commitsToday = 0
	}
	metrics.CommitsToday = commitsToday

	return metrics, nil
}

// GetEmployeeMRDetails returns detailed merge requests for an employee (current month)
func (g *GitLabCollector) GetEmployeeMRDetails(employee models.Employee) ([]models.MergeRequest, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	return g.GetEmployeeMRDetailsRange(employee, monthStart, monthEnd)
}

// GetEmployeeMRDetailsSince returns MRs since a specific date (no upper bound)
func (g *GitLabCollector) GetEmployeeMRDetailsSince(employee models.Employee, since time.Time) ([]models.MergeRequest, error) {
	return g.GetEmployeeMRDetailsRange(employee, since, time.Time{})
}

// GetEmployeeMRDetailsRange returns MRs in a date range
func (g *GitLabCollector) GetEmployeeMRDetailsRange(employee models.Employee, since time.Time, until time.Time) ([]models.MergeRequest, error) {
	now := time.Now()

	mrs, err := g.getUserMRsRange(employee.Email, since, until)
	if err != nil {
		return nil, err
	}

	var result []models.MergeRequest
	for _, mr := range mrs {
		created, _ := time.Parse(time.RFC3339, mr.CreatedAt)
		daysOpen := int(now.Sub(created).Hours() / 24)

		var reviewers []string
		for _, r := range mr.Reviewers {
			reviewers = append(reviewers, r.Name)
		}

		item := models.MergeRequest{
			ID:             mr.ID,
			IID:            mr.IID,
			Title:          mr.Title,
			State:          mr.State,
			URL:            mr.WebURL,
			Author:         mr.Author.Name,
			CreatedAt:      created,
			SourceBranch:   mr.SourceBranch,
			TargetBranch:   mr.TargetBranch,
			Project:        mr.References.Full,
			HasConflicts:   mr.HasConflicts,
			Reviewers:      reviewers,
			PipelineStatus: mr.PipelineStatus,
			DaysOpen:       daysOpen,
		}

		if mr.MergedAt != nil {
			if t, err := time.Parse(time.RFC3339, *mr.MergedAt); err == nil {
				item.MergedAt = &t
				item.DaysOpen = int(t.Sub(created).Hours() / 24)
			}
		}

		result = append(result, item)
	}

	return result, nil
}

func (g *GitLabCollector) getUserMRs(email string, since time.Time) ([]gitlabMR, error) {
	return g.getUserMRsRange(email, since, time.Time{})
}

func (g *GitLabCollector) getUserMRsRange(email string, since time.Time, until time.Time) ([]gitlabMR, error) {
	sinceStr := since.Format(time.RFC3339)
	username := extractUsername(email)
	endpoint := fmt.Sprintf("%s/api/v4/merge_requests?author_username=%s&created_after=%s&per_page=100&scope=all",
		g.cfg.GitLab.URL, url.QueryEscape(username), url.QueryEscape(sinceStr))

	// Add upper bound if specified
	if !until.IsZero() {
		untilStr := until.Format(time.RFC3339)
		endpoint += "&created_before=" + url.QueryEscape(untilStr)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.cfg.GitLab.Token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab MR request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab returned %d: %s", resp.StatusCode, string(body))
	}

	var mrs []gitlabMR
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return nil, err
	}

	return mrs, nil
}

// GetEmployeeMRDetailsRangeMulti fetches MRs for employee using all their known usernames
func (g *GitLabCollector) GetEmployeeMRDetailsRangeMulti(employee models.Employee, since time.Time, until time.Time) ([]models.MergeRequest, error) {
	// Collect all usernames to search
	usernames := []string{extractUsername(employee.Email)}
	if employee.GitlabUsername != "" && employee.GitlabUsername != usernames[0] {
		usernames = append(usernames, employee.GitlabUsername)
	}

	var allMRs []models.MergeRequest
	seen := make(map[int]bool)

	for _, username := range usernames {
		mrs, err := g.getUserMRsRangeByUsername(username, since, until)
		if err != nil {
			continue
		}
		now := time.Now()
		for _, mr := range mrs {
			if seen[mr.ID] {
				continue
			}
			seen[mr.ID] = true

			created, _ := time.Parse(time.RFC3339, mr.CreatedAt)
			daysOpen := int(now.Sub(created).Hours() / 24)
			var reviewers []string
			for _, r := range mr.Reviewers {
				reviewers = append(reviewers, r.Name)
			}
			item := models.MergeRequest{
				ID: mr.ID, IID: mr.IID, Title: mr.Title, State: mr.State,
				URL: mr.WebURL, Author: mr.Author.Name, CreatedAt: created,
				SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
				Project: mr.References.Full, HasConflicts: mr.HasConflicts,
				Reviewers: reviewers, PipelineStatus: mr.PipelineStatus, DaysOpen: daysOpen,
			}
			if mr.MergedAt != nil {
				if t, err := time.Parse(time.RFC3339, *mr.MergedAt); err == nil {
					item.MergedAt = &t
					item.DaysOpen = int(t.Sub(created).Hours() / 24)
				}
			}
			allMRs = append(allMRs, item)
		}
	}
	return allMRs, nil
}

func (g *GitLabCollector) getUserMRsRangeByUsername(username string, since time.Time, until time.Time) ([]gitlabMR, error) {
	sinceStr := since.Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v4/merge_requests?author_username=%s&created_after=%s&per_page=100&scope=all",
		g.cfg.GitLab.URL, url.QueryEscape(username), url.QueryEscape(sinceStr))
	if !until.IsZero() {
		endpoint += "&created_before=" + url.QueryEscape(until.Format(time.RFC3339))
	}
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("PRIVATE-TOKEN", g.cfg.GitLab.Token)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab returned %d", resp.StatusCode)
	}
	var mrs []gitlabMR
	json.NewDecoder(resp.Body).Decode(&mrs)
	return mrs, nil
}

func (g *GitLabCollector) getUserCommits(employee models.Employee, since time.Time) (int, error) {
	// Collect all usernames to search push events for
	usernames := []string{extractUsername(employee.Email)}
	if employee.GitlabUsername != "" && employee.GitlabUsername != usernames[0] {
		usernames = append(usernames, employee.GitlabUsername)
	}

	total := 0
	for _, username := range usernames {
		sinceStr := since.Format(time.RFC3339)
		endpoint := fmt.Sprintf("%s/api/v4/users/%s/events?action=pushed&after=%s&per_page=100",
			g.cfg.GitLab.URL, url.QueryEscape(username), url.QueryEscape(sinceStr))

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("PRIVATE-TOKEN", g.cfg.GitLab.Token)

		resp, err := g.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var events []gitlabEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			continue
		}
		total += len(events)
	}
	return total, nil
}

// extractUsername gets username part from email (e.g., "DAltybassarova" from "DAltybassarova@Fortebank.com")
func extractUsername(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

type gitlabMR struct {
	ID             int           `json:"id"`
	IID            int           `json:"iid"`
	Title          string        `json:"title"`
	State          string        `json:"state"`
	CreatedAt      string        `json:"created_at"`
	MergedAt       *string       `json:"merged_at"`
	WebURL         string        `json:"web_url"`
	Reviewers      []gitlabUser  `json:"reviewers"`
	Author         gitlabUser    `json:"author"`
	SourceBranch   string        `json:"source_branch"`
	TargetBranch   string        `json:"target_branch"`
	ProjectPath    string        `json:"-"`
	HasConflicts   bool          `json:"has_conflicts"`
	PipelineStatus string        `json:"pipeline_status"`
	References     struct {
		Full string `json:"full"`
	} `json:"references"`
}

type gitlabUser struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

type gitlabEvent struct {
	ActionName string `json:"action_name"`
	CreatedAt  string `json:"created_at"`
	PushData   *struct {
		CommitTitle string `json:"commit_title"`
		RefType     string `json:"ref_type"`
		Ref         string `json:"ref"`
		CommitCount int    `json:"commit_count"`
	} `json:"push_data"`
	ProjectID int `json:"project_id"`
	TargetTitle string `json:"target_title"`
}

type gitlabProject struct {
	PathWithNamespace string `json:"path_with_namespace"`
}

// PushEventInfo represents a single push event for the API response
type PushEventInfo struct {
	Project     string `json:"project"`
	Date        string `json:"date"`
	CommitTitle string `json:"commit_title"`
}

// GetUserPushEvents returns push events for an employee since a given date
func (g *GitLabCollector) GetUserPushEvents(employee models.Employee, since time.Time) ([]PushEventInfo, error) {
	usernames := []string{extractUsername(employee.Email)}
	if employee.GitlabUsername != "" && employee.GitlabUsername != usernames[0] {
		usernames = append(usernames, employee.GitlabUsername)
	}

	var result []PushEventInfo
	projectCache := make(map[int]string)

	for _, username := range usernames {
		sinceStr := since.Format(time.RFC3339)
		endpoint := fmt.Sprintf("%s/api/v4/users/%s/events?action=pushed&after=%s&per_page=100&sort=desc",
			g.cfg.GitLab.URL, url.QueryEscape(username), url.QueryEscape(sinceStr))

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("PRIVATE-TOKEN", g.cfg.GitLab.Token)

		resp, err := g.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var events []gitlabEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			continue
		}

		for _, ev := range events {
			commitTitle := ""
			if ev.PushData != nil {
				commitTitle = ev.PushData.CommitTitle
			}

			// Resolve project name
			projectName := ""
			if ev.ProjectID > 0 {
				if cached, ok := projectCache[ev.ProjectID]; ok {
					projectName = cached
				} else {
					projectName = g.resolveProjectPath(ev.ProjectID)
					projectCache[ev.ProjectID] = projectName
				}
			}

			// Parse date
			date := ""
			if t, err := time.Parse(time.RFC3339, ev.CreatedAt); err == nil {
				date = t.Format("02.01.2006 15:04")
			}

			result = append(result, PushEventInfo{
				Project:     projectName,
				Date:        date,
				CommitTitle: commitTitle,
			})
		}
	}
	return result, nil
}

func (g *GitLabCollector) resolveProjectPath(projectID int) string {
	endpoint := fmt.Sprintf("%s/api/v4/projects/%d?simple=true", g.cfg.GitLab.URL, projectID)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("PRIVATE-TOKEN", g.cfg.GitLab.Token)
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Sprintf("project/%d", projectID)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("project/%d", projectID)
	}
	var p gitlabProject
	json.NewDecoder(resp.Body).Decode(&p)
	if p.PathWithNamespace != "" {
		return p.PathWithNamespace
	}
	return fmt.Sprintf("project/%d", projectID)
}
