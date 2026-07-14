package collector

import (
	"testing"
)

func TestExtractUsername(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"DAltybassarova@Fortebank.com", "DAltybassarova"},
		{"SAbdikhalyk@Fortebank.com", "SAbdikhalyk"},
		{"DiaAubakirov@Fortebank.com", "DiaAubakirov"},
		{"AUteshkaliyev@Fortebank.com", "AUteshkaliyev"},
		{"AAbdukarimova@Fortebank.com", "AAbdukarimova"},
		{"user@domain.com", "user"},
		{"nodomain", "nodomain"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := extractUsername(tt.email)
			if result != tt.expected {
				t.Errorf("extractUsername(%q) = %q, want %q", tt.email, result, tt.expected)
			}
		})
	}
}

func TestExtractConfUsername(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"DAltybassarova@Fortebank.com", "DAltybassarova"},
		{"SAbdikhalyk@Fortebank.com", "SAbdikhalyk"},
		{"user", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := extractConfUsername(tt.email)
			if result != tt.expected {
				t.Errorf("extractConfUsername(%q) = %q, want %q", tt.email, result, tt.expected)
			}
		})
	}
}
