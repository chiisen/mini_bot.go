package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Agents    AgentsConfig           `json:"agents"`
	Providers map[string]ModelConfig `json:"providers"`
	Channels  ChannelsConfig         `json:"channels"`
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

type AgentDefaults struct {
	Workspace           string  `json:"workspace"`
	Model               string  `json:"model"`
	MaxTokens           int     `json:"maxTokens"`
	Temperature         float64 `json:"temperature"`
	MaxToolIterations   int     `json:"maxToolIterations"`
	MemoryWindow        int     `json:"memoryWindow"`
	RestrictToWorkspace bool    `json:"restrictToWorkspace"`
}

type ModelConfig struct {
	APIKey  string `json:"apiKey"`
	APIBase string `json:"apiBase,omitempty"`

	// internal fields
	Vendor string `json:"-"`
	Model  string `json:"-"`
}

type ChannelsConfig struct {
	Telegram TelegramConfig `json:"telegram"`
}

type TelegramConfig struct {
	Enabled   bool     `json:"enabled"`
	Token     string   `json:"botToken"`
	AllowFrom []string `json:"allow_from"`
}

// Load loads the configuration with a 3-tier priority: Default -> JSON -> Env
func Load(configPath string) (*Config, error) {
	cfg := &Config{}
	
	// 1. Apply defaults
	applyDefaults(cfg)

	// Expand ~ to user home dir for configPath
	expandedPath := expandHome(configPath)

	// 2. Load from JSON if exists
	if _, err := os.Stat(expandedPath); err == nil {
		data, err := os.ReadFile(expandedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// 3. Apply Environment Variable overrides
	applyEnvOverrides(cfg)
	
	// Expand workspace path
	cfg.Agents.Defaults.Workspace = expandHome(cfg.Agents.Defaults.Workspace)

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	cfg.Agents.Defaults.Workspace = DefaultWorkspace
	cfg.Agents.Defaults.Model = DefaultModel
	cfg.Agents.Defaults.MaxTokens = DefaultMaxTokens
	cfg.Agents.Defaults.Temperature = DefaultTemperature
	cfg.Agents.Defaults.MaxToolIterations = DefaultMaxToolIterations
	cfg.Agents.Defaults.RestrictToWorkspace = DefaultRestrictToWS
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("MINIBOT_AGENTS_DEFAULTS_WORKSPACE"); v != "" {
		cfg.Agents.Defaults.Workspace = v
	}
	if v := os.Getenv("MINIBOT_AGENTS_DEFAULTS_MODEL"); v != "" {
		cfg.Agents.Defaults.Model = v
	}
	if v := os.Getenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOKENS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Agents.Defaults.MaxTokens = val
		}
	}
	if v := os.Getenv("MINIBOT_AGENTS_DEFAULTS_TEMPERATURE"); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Agents.Defaults.Temperature = val
		}
	}
	if v := os.Getenv("MINIBOT_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			cfg.Agents.Defaults.MaxToolIterations = val
		}
	}
	if v := os.Getenv("MINIBOT_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE"); v != "" {
		if val, err := strconv.ParseBool(v); err == nil {
			cfg.Agents.Defaults.RestrictToWorkspace = val
		}
	}
}

// FindModel returns the ModelConfig for the given model string (e.g., "minimax/MiniMax-M2.5").
func (c *Config) FindModel(modelDef string) (*ModelConfig, error) {
	parts := strings.SplitN(modelDef, "/", 2)
	vendor := "openai" // Default fallback
	
	if len(parts) == 2 {
		vendor = strings.ToLower(parts[0])
	}

	if p, ok := c.Providers[vendor]; ok {
		p.Vendor = vendor
		p.Model = modelDef // store the full model string to pass down to factory
		return &p, nil
	}
	
	return nil, fmt.Errorf("model not found: %s", modelDef)
}

// expandHome resolves "~" to the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
