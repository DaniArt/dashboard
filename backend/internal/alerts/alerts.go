package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/config"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
)

type AlertEngine struct {
	cfg *config.Config
}

func NewAlertEngine(cfg *config.Config) *AlertEngine {
	return &AlertEngine{cfg: cfg}
}

// isActiveStatus checks if a task status should be monitored for alerts
func isActiveStatus(status string) bool {
	s := strings.ToLower(status)
	activeStatuses := []string{"открытый", "на анализе", "в работе", "анализ", "analysis", "analytics"}
	for _, active := range activeStatuses {
		if strings.Contains(s, active) {
			return true
		}
	}
	return false
}

// CheckAlerts generates alerts based on current task data
func (a *AlertEngine) CheckAlerts(employee models.Employee, issues []models.JiraIssue, gitlab models.GitLabMetrics) []models.Alert {
	var alerts []models.Alert

	now := time.Now()

	for _, issue := range issues {
		// Only check active statuses
		if !isActiveStatus(issue.Status) {
			continue
		}

		// Check stale tasks (in same status > N days)
		daysInStatus := int(now.Sub(issue.StatusSince).Hours() / 24)
		if daysInStatus >= a.cfg.Alerts.StaleTaskDays {
			severity := "warning"
			if daysInStatus >= a.cfg.Alerts.StaleTaskDays*2 {
				severity = "critical"
			}
			alerts = append(alerts, models.Alert{
				ID:           fmt.Sprintf("stale_%s", issue.Key),
				Employee:     employee.Name,
				Type:         "stale_task",
				Severity:     severity,
				Message:      fmt.Sprintf("Задача %s в статусе \"%s\" уже %d дней", issue.Key, issue.Status, daysInStatus),
				TaskKey:      issue.Key,
				TaskURL:      issue.URL,
				CreatedAt:    now,
				DaysInStatus: daysInStatus,
			})
		}
	}

	// Check no activity
	if gitlab.CommitsMonth == 0 && gitlab.MRsCreatedMonth == 0 && len(employee.GitLabGroups) > 0 {
		alerts = append(alerts, models.Alert{
			ID:        fmt.Sprintf("no_git_activity_%s", employee.Email),
			Employee:  employee.Name,
			Type:      "no_activity",
			Severity:  "warning",
			Message:   fmt.Sprintf("Нет активности в GitLab за текущий месяц"),
			CreatedAt: now,
		})
	}

	// Check MRs without review
	if gitlab.MRsWithoutReview > 0 {
		alerts = append(alerts, models.Alert{
			ID:        fmt.Sprintf("mr_no_review_%s", employee.Email),
			Employee:  employee.Name,
			Type:      "mr_no_review",
			Severity:  "warning",
			Message:   fmt.Sprintf("%d MR без назначенного ревьюера", gitlab.MRsWithoutReview),
			CreatedAt: now,
		})
	}

	return alerts
}
