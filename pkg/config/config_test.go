package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRequestTimeout 驗證單次 LLM 請求的逾時換算，並確認異常值會退回預設值
// （http.Client 的 0 代表「不設逾時」，不能讓它流到 provider）。
func TestRequestTimeout(t *testing.T) {
	cases := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"使用設定值", 15, 15 * time.Second},
		{"未設定時使用預設", 0, DefaultRequestTimeoutSeconds * time.Second},
		{"負值時使用預設", -5, DefaultRequestTimeoutSeconds * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Agents.Defaults.RequestTimeoutSeconds = tc.seconds

			if got := cfg.requestTimeout(); got != tc.want {
				t.Errorf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

// TestFindModel_SetsRequestTimeout 驗證逾時會隨選定的模型傳遞給 provider。
func TestFindModel_SetsRequestTimeout(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{Defaults: AgentDefaults{RequestTimeoutSeconds: 12}},
		Providers: map[string]ModelConfig{
			"openai": {APIKey: "k"},
		},
	}

	modelCfg, err := cfg.FindModel("openai/gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if modelCfg.RequestTimeout != 12*time.Second {
		t.Errorf("expected 12s, got %s", modelCfg.RequestTimeout)
	}
}

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
				"maxToolIterations": 30
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
	defer func() {
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_WORKSPACE")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_MODEL")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOKENS")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_TEMPERATURE")
		os.Unsetenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS")
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
