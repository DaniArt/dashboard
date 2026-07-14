package config

import (
	"os"
	"strings"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Jira       JiraConfig       `yaml:"jira"`
	Confluence ConfluenceConfig `yaml:"confluence"`
	GitLab     GitLabConfig     `yaml:"gitlab"`
	Alerts     AlertsConfig     `yaml:"alerts"`
	Employees  []models.Employee `yaml:"employees"`
	AnalysisProject string       `yaml:"analysis_project"`
	Workflows  WorkflowsConfig  `yaml:"workflows"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

type JiraConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type ConfluenceConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type GitLabConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type AlertsConfig struct {
	StaleTaskDays    int `yaml:"stale_task_days"`
	NoAssigneeHours  int `yaml:"no_assignee_hours"`
	MRNoReviewHours  int `yaml:"mr_no_review_hours"`
	NoActivityDays   int `yaml:"no_activity_days"`
}

type WorkflowConfig struct {
	Types      []string `yaml:"types,omitempty"`
	InProgress []string `yaml:"in_progress"`
	Review     []string `yaml:"review,omitempty"`
	Done       []string `yaml:"done"`
}

type WorkflowsConfig struct {
	Analysis        WorkflowConfig `yaml:"analysis"`
	UserStory       WorkflowConfig `yaml:"user_story"`
	UserStoryDomains WorkflowConfig `yaml:"user_story_domains"`
	VedTrf          WorkflowConfig `yaml:"ved_trf"`
	Support         WorkflowConfig `yaml:"support"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables
	content := string(data)
	content = expandEnvVars(content)

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.PollInterval == 0 {
		cfg.Server.PollInterval = 5 * time.Minute
	}
	if cfg.Alerts.StaleTaskDays == 0 {
		cfg.Alerts.StaleTaskDays = 5
	}

	return &cfg, nil
}

func expandEnvVars(s string) string {
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			s = strings.ReplaceAll(s, "${"+parts[0]+"}", parts[1])
		}
	}
	return s
}
