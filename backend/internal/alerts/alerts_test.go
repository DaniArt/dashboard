package alerts

import (
	"testing"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/config"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
)

func TestCheckAlerts_StaleTask(t *testing.T) {
	cfg := &config.Config{
		Alerts: config.AlertsConfig{StaleTaskDays: 5},
	}
	engine := NewAlertEngine(cfg)

	emp := models.Employee{Name: "Test", Email: "test@test.com"}
	now := time.Now()

	issues := []models.JiraIssue{
		{Key: "PMB-1", Status: "В работе", StatusSince: now.AddDate(0, 0, -10), URL: "http://jira/PMB-1"},
		{Key: "PMB-2", Status: "В работе", StatusSince: now.AddDate(0, 0, -2), URL: "http://jira/PMB-2"},
	}
	gitMetrics := models.GitLabMetrics{MRsCreatedMonth: 1}

	alerts := engine.CheckAlerts(emp, issues, gitMetrics)

	// Should have 1 stale task alert (PMB-1 is 10 days, PMB-2 is 2 days)
	staleAlerts := 0
	for _, a := range alerts {
		if a.Type == "stale_task" {
			staleAlerts++
		}
	}
	if staleAlerts != 1 {
		t.Errorf("Expected 1 stale task alert, got %d", staleAlerts)
	}
}

func TestCheckAlerts_CriticalSeverity(t *testing.T) {
	cfg := &config.Config{
		Alerts: config.AlertsConfig{StaleTaskDays: 5},
	}
	engine := NewAlertEngine(cfg)

	emp := models.Employee{Name: "Test", Email: "test@test.com"}
	now := time.Now()

	issues := []models.JiraIssue{
		{Key: "PMB-1", Status: "В работе", StatusSince: now.AddDate(0, 0, -15), URL: "http://jira/PMB-1"},
	}
	gitMetrics := models.GitLabMetrics{MRsCreatedMonth: 1}

	alerts := engine.CheckAlerts(emp, issues, gitMetrics)

	// 15 days > 5*2 = critical
	for _, a := range alerts {
		if a.Type == "stale_task" && a.Severity != "critical" {
			t.Errorf("Expected critical severity for 15-day stale task, got %s", a.Severity)
		}
	}
}

func TestCheckAlerts_NoGitActivity(t *testing.T) {
	cfg := &config.Config{
		Alerts: config.AlertsConfig{StaleTaskDays: 5},
	}
	engine := NewAlertEngine(cfg)

	emp := models.Employee{Name: "Test", Email: "test@test.com", GitLabGroups: []string{"domains"}}
	issues := []models.JiraIssue{}
	gitMetrics := models.GitLabMetrics{MRsCreatedMonth: 0, CommitsMonth: 0}

	alerts := engine.CheckAlerts(emp, issues, gitMetrics)

	hasNoActivity := false
	for _, a := range alerts {
		if a.Type == "no_activity" {
			hasNoActivity = true
		}
	}
	if !hasNoActivity {
		t.Error("Expected no_activity alert for employee with GitLab groups but no activity")
	}
}

func TestCheckAlerts_NoAlerts(t *testing.T) {
	cfg := &config.Config{
		Alerts: config.AlertsConfig{StaleTaskDays: 5},
	}
	engine := NewAlertEngine(cfg)

	emp := models.Employee{Name: "Test", Email: "test@test.com"}
	now := time.Now()
	issues := []models.JiraIssue{
		{Key: "PMB-1", Status: "В работе", StatusSince: now.AddDate(0, 0, -2)},
	}
	gitMetrics := models.GitLabMetrics{MRsCreatedMonth: 3, CommitsMonth: 10}

	alerts := engine.CheckAlerts(emp, issues, gitMetrics)

	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts, got %d: %+v", len(alerts), alerts)
	}
}
