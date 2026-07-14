package models

import "time"

// Employee represents a team member
type Employee struct {
	Name           string   `json:"name" yaml:"name"`
	Email          string   `json:"email" yaml:"email"`
	GitlabUsername string   `json:"gitlab_username,omitempty" yaml:"gitlab_username"`
	JiraProjects   []string `json:"jira_projects" yaml:"jira_projects"`
	GitLabGroups   []string `json:"gitlab_groups" yaml:"gitlab_groups"`
	GitLabRepos    []string `json:"gitlab_repos,omitempty" yaml:"gitlab_repos"`
	Role           string   `json:"role" yaml:"role"`
	ART            string   `json:"art,omitempty" yaml:"art"`
	Team           string   `json:"team,omitempty" yaml:"team"`
	ShowInMR       bool     `json:"show_in_mr" yaml:"show_in_mr"`
}

// TaskMetrics holds task-related metrics for an employee
type TaskMetrics struct {
	ActiveTasks      int            `json:"active_tasks"`      // Тестирование, Готово к внедрению, Регрессионное тестирование
	BacklogTasks     int            `json:"backlog_tasks"`     // Готово к тестированию
	ClosedTasks      int            `json:"closed_tasks"`      // Готово к релизу, Выполнено, Готово
	TotalTasks       int            `json:"total_tasks"`
	CompletedToday   int            `json:"completed_today"`
	CompletedMonth   int            `json:"completed_month"`
	AvgCycleTimeDays float64        `json:"avg_cycle_time_days"`
	ByStatus         map[string]int `json:"by_status"`
	ByType           map[string]int `json:"by_type"`
	OverdueTasks     int            `json:"overdue_tasks"`
	StaleTasks       int            `json:"stale_tasks"` // > 3 days in active statuses
}

// ConfluenceMetrics holds documentation quality metrics
type ConfluenceMetrics struct {
	PagesCreatedMonth  int     `json:"pages_created_month"`
	PagesUpdatedMonth  int     `json:"pages_updated_month"`
	PagesCreatedToday  int     `json:"pages_created_today"`
	TotalPages         int     `json:"total_pages"`
	StalePages         int     `json:"stale_pages"` // not updated > 30 days
	QualityScore       float64 `json:"quality_score"` // 0-100
}

// GitLabMetrics holds code/proto contract quality metrics
type GitLabMetrics struct {
	MRsCreatedMonth  int     `json:"mrs_created_month"`
	MRsMergedMonth   int     `json:"mrs_merged_month"`
	MRsOpen          int     `json:"mrs_open"`
	CommitsMonth     int     `json:"commits_month"`
	CommitsToday     int     `json:"commits_today"`
	AvgReviewTimeH   float64 `json:"avg_review_time_hours"`
	MRsWithoutReview int     `json:"mrs_without_review"`
}

// Alert represents a notification about an issue
type Alert struct {
	ID         string    `json:"id"`
	Employee   string    `json:"employee"`
	Type       string    `json:"type"` // stale_task, no_activity, mr_no_review, overdue
	Severity   string    `json:"severity"` // warning, critical
	Message    string    `json:"message"`
	TaskKey    string    `json:"task_key,omitempty"`
	TaskURL    string    `json:"task_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	DaysInStatus int    `json:"days_in_status,omitempty"`
}

// EmployeeDashboard is the complete data for one employee card
type EmployeeDashboard struct {
	Employee    Employee          `json:"employee"`
	Tasks       TaskMetrics       `json:"tasks"`
	Confluence  ConfluenceMetrics `json:"confluence"`
	GitLab      GitLabMetrics     `json:"gitlab"`
	Alerts      []Alert           `json:"alerts"`
	LastUpdated time.Time         `json:"last_updated"`
}

// DashboardResponse is the full API response
type DashboardResponse struct {
	Employees   []EmployeeDashboard `json:"employees"`
	Summary     TeamSummary         `json:"summary"`
	Alerts      []Alert             `json:"alerts"`
	LastUpdated time.Time           `json:"last_updated"`
}

// TeamSummary holds aggregated team metrics
type TeamSummary struct {
	TotalActiveTasks    int     `json:"total_active_tasks"`
	TotalCompletedToday int     `json:"total_completed_today"`
	TotalCompletedMonth int     `json:"total_completed_month"`
	TotalAlerts         int     `json:"total_alerts"`
	CriticalAlerts      int     `json:"critical_alerts"`
	TeamAvgCycleTime    float64 `json:"team_avg_cycle_time_days"`
	TotalMRsMonth       int     `json:"total_mrs_month"`
	TotalPagesMonth     int     `json:"total_pages_month"`
}

// JiraIssue represents a simplified Jira issue
type JiraIssue struct {
	Key          string    `json:"key"`
	Summary      string    `json:"summary"`
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	Assignee     string    `json:"assignee"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
	StatusSince  time.Time `json:"status_since"`
	Project      string    `json:"project"`
	URL          string    `json:"url"`
}

// JiraComment represents a comment on a Jira issue
type JiraComment struct {
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
}

// MergeRequest represents a GitLab merge request with details
type MergeRequest struct {
	ID          int       `json:"id"`
	IID         int       `json:"iid"`
	Title       string    `json:"title"`
	State       string    `json:"state"` // opened, merged, closed
	URL         string    `json:"url"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
	MergedAt    *time.Time `json:"merged_at,omitempty"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Project     string    `json:"project"`
	HasConflicts bool     `json:"has_conflicts"`
	Reviewers   []string  `json:"reviewers"`
	PipelineStatus string `json:"pipeline_status"` // success, failed, running, pending
	DaysOpen    int       `json:"days_open"`
}

// ConfluencePage represents a Confluence page with details
type ConfluencePage struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Space           string    `json:"space"`
	SpaceName       string    `json:"space_name"`
	URL             string    `json:"url"`
	Creator         string    `json:"creator"`
	LastUpdated     time.Time `json:"last_updated"`
	Version         int       `json:"version"`
	BodyLength      int       `json:"body_length"`
	DaysSinceUpdate int       `json:"days_since_update"`
	Changes         string    `json:"changes"` // brief description of what changed
}

// MRDetailResponse is the response for merge requests page
type MRDetailResponse struct {
	Employee    string         `json:"employee"`
	MRs         []MergeRequest `json:"mrs"`
	Metrics     GitLabMetrics  `json:"metrics"`
	Conclusion  string         `json:"conclusion"`
}

// ConfluenceDetailResponse is the response for confluence page
type ConfluenceDetailResponse struct {
	Employee    string           `json:"employee"`
	Pages       []ConfluencePage `json:"pages"`
	Metrics     ConfluenceMetrics `json:"metrics"`
	Conclusion  string           `json:"conclusion"`
}
