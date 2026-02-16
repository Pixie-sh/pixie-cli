package shared

import (
	"sort"
	"testing"
)

func TestIsValidSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"user_management", true},
		{"users", true},
		{"a", true},
		{"my_service_v2", true},
		{"user2_service", true},

		// Invalid cases
		{"", false},
		{"UserManagement", false},  // uppercase
		{"user-management", false}, // hyphen
		{"_users", false},          // leading underscore
		{"users_", false},          // trailing underscore
		{"2users", false},          // leading digit
		{"user management", false}, // space
		{"user.management", false}, // dot
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsValidSnakeCase(tt.input); got != tt.want {
				t.Errorf("IsValidSnakeCase(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"users", true},
		{"UserManagement", true},
		{"myService2", true},
		{"a", true},
		{"A", true},

		// Invalid cases
		{"", false},
		{"2users", false},          // leading digit
		{"user_management", false}, // underscore
		{"user-name", false},       // hyphen
		{"user name", false},       // space
		{"user.name", false},       // dot
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsValidIdentifier(tt.input); got != tt.want {
				t.Errorf("IsValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFeatures(t *testing.T) {
	tests := []struct {
		name         string
		featureStr   string
		templateType string
		wantKeys     []string
	}{
		{
			name:         "minimal template",
			featureStr:   "",
			templateType: "minimal",
			wantKeys:     []string{"metrics"},
		},
		{
			name:         "standard template",
			featureStr:   "",
			templateType: "standard",
			wantKeys:     []string{"database", "metrics"},
		},
		{
			name:         "full template",
			featureStr:   "",
			templateType: "full",
			wantKeys:     []string{"database", "metrics", "auth", "cache", "tokens", "events", "notifications", "backoffice", "validation", "adapters", "apis"},
		},
		{
			name:         "custom features override",
			featureStr:   "custom1, custom2",
			templateType: "minimal",
			wantKeys:     []string{"metrics", "custom1", "custom2"},
		},
		{
			name:         "no template type with features",
			featureStr:   "database,cache",
			templateType: "",
			wantKeys:     []string{"database", "cache"},
		},
		{
			name:         "empty everything",
			featureStr:   "",
			templateType: "",
			wantKeys:     nil,
		},
		{
			name:         "whitespace handling",
			featureStr:   " auth , cache , ",
			templateType: "",
			wantKeys:     []string{"auth", "cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFeatures(tt.featureStr, tt.templateType)

			if tt.wantKeys == nil {
				if len(got) != 0 {
					t.Errorf("ParseFeatures() = %v, want empty", got)
				}
				return
			}

			for _, key := range tt.wantKeys {
				if !got[key] {
					t.Errorf("ParseFeatures() missing key %q", key)
				}
			}
		})
	}
}

func TestResolveFeatureDependencies(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]bool
		wantKeys []string
	}{
		{
			name:     "auth requires tokens",
			input:    map[string]bool{"auth": true},
			wantKeys: []string{"auth", "tokens"},
		},
		{
			name:     "backoffice requires auth and tokens",
			input:    map[string]bool{"backoffice": true},
			wantKeys: []string{"backoffice", "auth", "tokens"},
		},
		{
			name:     "events requires cache",
			input:    map[string]bool{"events": true},
			wantKeys: []string{"events", "cache"},
		},
		{
			name:     "notifications requires events and cache",
			input:    map[string]bool{"notifications": true},
			wantKeys: []string{"notifications", "events", "cache"},
		},
		{
			name:     "no dependencies",
			input:    map[string]bool{"database": true, "metrics": true},
			wantKeys: []string{"database", "metrics"},
		},
		{
			name:     "empty",
			input:    map[string]bool{},
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveFeatureDependencies(tt.input)

			for _, key := range tt.wantKeys {
				if !got[key] {
					t.Errorf("ResolveFeatureDependencies() missing key %q", key)
				}
			}
		})
	}
}

func TestEnabledFeatures(t *testing.T) {
	features := map[string]bool{
		"auth":     true,
		"cache":    true,
		"database": false,
		"metrics":  true,
	}

	got := EnabledFeatures(features)
	sort.Strings(got)

	want := []string{"auth", "cache", "metrics"}
	if len(got) != len(want) {
		t.Fatalf("EnabledFeatures() returned %d items, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("EnabledFeatures()[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestFeaturesListString(t *testing.T) {
	tests := []struct {
		name     string
		features map[string]bool
		want     string
	}{
		{
			name:     "empty features",
			features: map[string]bool{},
			want:     "none",
		},
		{
			name:     "single feature",
			features: map[string]bool{"auth": true},
			want:     "auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FeaturesListString(tt.features)
			if tt.name == "empty features" && got != "none" {
				t.Errorf("FeaturesListString() = %q, want %q", got, "none")
			}
			if tt.name == "single feature" && got != "auth" {
				t.Errorf("FeaturesListString() = %q, want %q", got, "auth")
			}
		})
	}
}

func TestNewTemplateData(t *testing.T) {
	td := NewTemplateData()

	if td.Port != 8080 {
		t.Errorf("Port = %d, want 8080", td.Port)
	}
	if td.MetricsPort != 9090 {
		t.Errorf("MetricsPort = %d, want 9090", td.MetricsPort)
	}
	if td.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
	if td.MigrationTimestamp == "" {
		t.Error("MigrationTimestamp should not be empty")
	}
	if td.Features == nil {
		t.Error("Features map should be initialized")
	}
	if td.DatabaseHost != "localhost" {
		t.Errorf("DatabaseHost = %q, want %q", td.DatabaseHost, "localhost")
	}
	if td.DatabasePort != 5432 {
		t.Errorf("DatabasePort = %d, want 5432", td.DatabasePort)
	}
	if td.RedisHost != "localhost" {
		t.Errorf("RedisHost = %q, want %q", td.RedisHost, "localhost")
	}
	if td.RedisPort != 6379 {
		t.Errorf("RedisPort = %d, want 6379", td.RedisPort)
	}
}
