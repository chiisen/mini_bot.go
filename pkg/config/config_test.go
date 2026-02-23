package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_WithDefaults(t *testing.T) {
	// Test loading with no config file - should use defaults
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After loading, workspace is expanded from ~/.minibot.go/workspace to actual path
	home, _ := os.UserHomeDir()
	expectedWorkspace := home + "/.minibot.go/workspace"

	if cfg.Agents.Defaults.Workspace != expectedWorkspace {
		t.Errorf("expected workspace %s, got %s", expectedWorkspace, cfg.Agents.Defaults.Workspace)
	}

	if cfg.Agents.Defaults.Model != DefaultModel {
		t.Errorf("expected model %s, got %s", DefaultModel, cfg.Agents.Defaults.Model)
	}

	if cfg.Agents.Defaults.MaxTokens != DefaultMaxTokens {
		t.Errorf("expected maxTokens %d, got %d", DefaultMaxTokens, cfg.Agents.Defaults.MaxTokens)
	}

	if cfg.Agents.Defaults.Temperature != DefaultTemperature {
		t.Errorf("expected temperature %v, got %v", DefaultTemperature, cfg.Agents.Defaults.Temperature)
	}

	if cfg.Agents.Defaults.MaxToolIterations != DefaultMaxToolIterations {
		t.Errorf("expected maxToolIterations %d, got %d", DefaultMaxToolIterations, cfg.Agents.Defaults.MaxToolIterations)
	}

	if !cfg.Agents.Defaults.RestrictToWorkspace {
		t.Error("expected restrictToWorkspace to be true by default")
	}
}

func TestLoad_WithJSONFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"agents": {
			"defaults": {
				"workspace": "/custom/workspace",
				"model": "minimax/MiniMax-M2.5",
				"maxTokens": 16384,
				"temperature": 0.9,
				"maxToolIterations": 30,
				"restrictToWorkspace": false
			}
		},
		"providers": {
			"minimax": {
				"apiKey": "test-key",
				"apiBase": "https://api.minimax.io/v1"
			}
		},
		"channels": {
			"telegram": {
				"enabled": true,
				"botToken": "test-token",
				"allow_from": ["123456"]
			}
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Agents.Defaults.Workspace != "/custom/workspace" {
		t.Errorf("expected workspace /custom/workspace, got %s", cfg.Agents.Defaults.Workspace)
	}

	if cfg.Agents.Defaults.Model != "minimax/MiniMax-M2.5" {
		t.Errorf("expected model minimax/MiniMax-M2.5, got %s", cfg.Agents.Defaults.Model)
	}

	if cfg.Agents.Defaults.MaxTokens != 16384 {
		t.Errorf("expected maxTokens 16384, got %d", cfg.Agents.Defaults.MaxTokens)
	}

	if cfg.Agents.Defaults.Temperature != 0.9 {
		t.Errorf("expected temperature 0.9, got %v", cfg.Agents.Defaults.Temperature)
	}

	if cfg.Agents.Defaults.MaxToolIterations != 30 {
		t.Errorf("expected maxToolIterations 30, got %d", cfg.Agents.Defaults.MaxToolIterations)
	}

	if cfg.Agents.Defaults.RestrictToWorkspace {
		t.Error("expected restrictToWorkspace to be false")
	}

	// Check provider
	provider, ok := cfg.Providers["minimax"]
	if !ok {
		t.Error("expected minimax provider")
	}
	if provider.APIKey != "test-key" {
		t.Errorf("expected apiKey test-key, got %s", provider.APIKey)
	}
	if provider.APIBase != "https://api.minimax.io/v1" {
		t.Errorf("expected apiBase https://api.minimax.io/v1, got %s", provider.APIBase)
	}

	// Check telegram
	if !cfg.Channels.Telegram.Enabled {
		t.Error("expected telegram enabled")
	}
	if cfg.Channels.Telegram.Token != "test-token" {
		t.Errorf("expected token test-token, got %s", cfg.Channels.Telegram.Token)
	}
	if len(cfg.Channels.Telegram.AllowFrom) != 1 || cfg.Channels.Telegram.AllowFrom[0] != "123456" {
		t.Errorf("expected allow_from [123456], got %v", cfg.Channels.Telegram.AllowFrom)
	}
}

func TestLoad_WithEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("MINIBOT_AGENTS_DEFAULTS_WORKSPACE", "/env/workspace")
	os.Setenv("MINIBOT_AGENTS_DEFAULTS_MODEL", "openai/gpt-4")
	os.Setenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOKENS", "4096")
	os.Setenv("MINIBOT_AGENTS_DEFAULTS_TEMPERATURE", "0.5")
	os.Setenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS", "10")
	os.Setenv("MINIBOT_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE", "false")
	defer func() {
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_WORKSPACE")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_MODEL")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOKENS")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_TEMPERATURE")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE")
	}()

	cfg, err := Load("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Agents.Defaults.Workspace != "/env/workspace" {
		t.Errorf("expected workspace /env/workspace, got %s", cfg.Agents.Defaults.Workspace)
	}

	if cfg.Agents.Defaults.Model != "openai/gpt-4" {
		t.Errorf("expected model openai/gpt-4, got %s", cfg.Agents.Defaults.Model)
	}

	if cfg.Agents.Defaults.MaxTokens != 4096 {
		t.Errorf("expected maxTokens 4096, got %d", cfg.Agents.Defaults.MaxTokens)
	}

	if cfg.Agents.Defaults.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %v", cfg.Agents.Defaults.Temperature)
	}

	if cfg.Agents.Defaults.MaxToolIterations != 10 {
		t.Errorf("expected maxToolIterations 10, got %d", cfg.Agents.Defaults.MaxToolIterations)
	}

	if cfg.Agents.Defaults.RestrictToWorkspace {
		t.Error("expected restrictToWorkspace to be false")
	}
}

func TestFindModel(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ModelConfig{
			"minimax": {
				APIKey:  "test-key",
				APIBase: "https://api.minimax.io/v1",
			},
			"openai": {
				APIKey:  "sk-test",
				APIBase: "https://api.openai.com/v1",
			},
		},
	}

	tests := []struct {
		modelDef   string
		wantVendor string
		wantErr    bool
	}{
		{"minimax/MiniMax-M2.5", "minimax", false},
		{"openai/gpt-4", "openai", false},
		{"gpt-4", "openai", false},         // default fallback
		{"unknown/model", "unknown", true}, // not found
	}

	for _, tt := range tests {
		t.Run(tt.modelDef, func(t *testing.T) {
			mc, err := cfg.FindModel(tt.modelDef)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if mc.Vendor != tt.wantVendor {
					t.Errorf("FindModel() vendor = %v, want %v", mc.Vendor, tt.wantVendor)
				}
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"~/", home},
		{"/absolute/path", "/absolute/path"},
		{"./relative/path", "./relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandHome(tt.input)
			if result != tt.expected {
				t.Errorf("expandHome(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
