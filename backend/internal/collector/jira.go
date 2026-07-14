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

type JiraCollector struct {
	cfg    *config.Config
	client *http.Client
}

func NewJiraCollector(cfg *config.Config) *JiraCollector {
	return &JiraCollector{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetEmployeeTasks returns all active tasks for an employee
func (j *JiraCollector) GetEmployeeTasks(employee models.Employee) ([]models.JiraIssue, error) {
	var allIssues []models.JiraIssue

	if len(employee.JiraProjects) == 0 {
		// Search across all projects by assignee
		jql := fmt.Sprintf(`assignee = "%s" AND resolution = Unresolved ORDER BY updated DESC`, employee.Email)
		issues, err := j.searchIssues(jql)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, issues...)
	} else {
		for _, project := range employee.JiraProjects {
			jql := fmt.Sprintf(`project = %s AND assignee = "%s" AND resolution = Unresolved ORDER BY updated DESC`, project, employee.Email)
			issues, err := j.searchIssues(jql)
			if err != nil {
				return nil, err
			}
			allIssues = append(allIssues, issues...)
		}
	}

	return allIssues, nil
}

// GetCompletedTasks returns tasks completed in a time range
func (j *JiraCollector) GetCompletedTasks(employee models.Employee, since time.Time) ([]models.JiraIssue, error) {
	sinceStr := since.Format("2006-01-02")

	var jql string
	if len(employee.JiraProjects) == 0 {
		jql = fmt.Sprintf(`assignee = "%s" AND resolved >= "%s" ORDER BY resolved DESC`, employee.Email, sinceStr)
	} else {
		projects := ""
		for i, p := range employee.JiraProjects {
			if i > 0 {
				projects += ", "
			}
			projects += p
		}
		jql = fmt.Sprintf(`project IN (%s) AND assignee = "%s" AND resolved >= "%s" ORDER BY resolved DESC`, projects, employee.Email, sinceStr)
	}

	return j.searchIssues(jql)
}

// GetAnalysisTasks returns tasks from NSA project for an employee
func (j *JiraCollector) GetAnalysisTasks(employee models.Employee) ([]models.JiraIssue, error) {
	jql := fmt.Sprintf(`project = %s AND assignee = "%s" ORDER BY updated DESC`, j.cfg.AnalysisProject, employee.Email)
	return j.searchIssues(jql)
}

// GetEmployeeTasksSince returns all tasks for an employee since a date (no upper bound)
func (j *JiraCollector) GetEmployeeTasksSince(employee models.Employee, since time.Time) ([]models.JiraIssue, error) {
	return j.GetEmployeeTasksRange(employee, since, time.Time{})
}

// GetEmployeeTasksRange returns tasks created OR updated in a specific date range across ALL projects
func (j *JiraCollector) GetEmployeeTasksRange(employee models.Employee, since time.Time, until time.Time) ([]models.JiraIssue, error) {
	sinceStr := since.Format("2006-01-02")

	var dateFilter string
	if until.IsZero() {
		dateFilter = fmt.Sprintf(`(created >= "%s" OR updated >= "%s")`, sinceStr, sinceStr)
	} else {
		untilStr := until.Format("2006-01-02")
		dateFilter = fmt.Sprintf(`((created >= "%s" AND created < "%s") OR (updated >= "%s" AND updated < "%s"))`, sinceStr, untilStr, sinceStr, untilStr)
	}

	// Search across ALL projects where employee is assignee
	jql := fmt.Sprintf(`assignee = "%s" AND %s ORDER BY updated DESC`, employee.Email, dateFilter)
	return j.searchIssues(jql)
}

func (j *JiraCollector) searchIssues(jql string) ([]models.JiraIssue, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/search?jql=%s&maxResults=100&fields=key,summary,status,issuetype,assignee,created,updated,project",
		j.cfg.Jira.URL, url.QueryEscape(jql))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+j.cfg.Jira.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira returned %d: %s", resp.StatusCode, string(body))
	}

	var result jiraSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var issues []models.JiraIssue
	for _, item := range result.Issues {
		issue := models.JiraIssue{
			Key:      item.Key,
			Summary:  item.Fields.Summary,
			Status:   item.Fields.Status.Name,
			Type:     item.Fields.IssueType.Name,
			Project:  item.Fields.Project.Key,
			URL:      fmt.Sprintf("%s/browse/%s", j.cfg.Jira.URL, item.Key),
		}

		if item.Fields.Assignee != nil {
			issue.Assignee = item.Fields.Assignee.EmailAddress
		}

		if t, err := time.Parse("2006-01-02T15:04:05.000-0700", item.Fields.Created); err == nil {
			issue.Created = t
		}
		if t, err := time.Parse("2006-01-02T15:04:05.000-0700", item.Fields.Updated); err == nil {
			issue.Updated = t
		}

		// Use Updated as approximate StatusSince (real value would need changelog)
		issue.StatusSince = issue.Updated

		issues = append(issues, issue)
	}

	return issues, nil
}

// Jira API response structures
type jiraSearchResult struct {
	Total  int         `json:"total"`
	Issues []jiraIssue `json:"issues"`
}

type jiraIssue struct {
	Key    string     `json:"key"`
	Fields jiraFields `json:"fields"`
}

type jiraFields struct {
	Summary   string        `json:"summary"`
	Status    jiraStatus    `json:"status"`
	IssueType jiraIssueType `json:"issuetype"`
	Assignee  *jiraUser     `json:"assignee"`
	Created   string        `json:"created"`
	Updated   string        `json:"updated"`
	Project   jiraProject   `json:"project"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraIssueType struct {
	Name string `json:"name"`
}

type jiraUser struct {
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

type jiraProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// GetIssueComments fetches comments for a specific Jira issue
func (j *JiraCollector) GetIssueComments(issueKey string) ([]models.JiraComment, error) {
	endpoint := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=comment",
		j.cfg.Jira.URL, issueKey)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+j.cfg.Jira.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira returned %d for comments on %s", resp.StatusCode, issueKey)
	}

	var result struct {
		Fields struct {
			Comment struct {
				Comments []struct {
					Author struct {
						DisplayName string `json:"displayName"`
					} `json:"author"`
					Body    string `json:"body"`
					Created string `json:"created"`
				} `json:"comments"`
			} `json:"comment"`
		} `json:"fields"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var comments []models.JiraComment
	for _, c := range result.Fields.Comment.Comments {
		created, _ := time.Parse("2006-01-02T15:04:05.000-0700", c.Created)
		comments = append(comments, models.JiraComment{
			Author:  c.Author.DisplayName,
			Body:    c.Body,
			Created: created,
		})
	}

	return comments, nil
}
