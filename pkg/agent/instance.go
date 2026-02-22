package agent

import (
	"fmt"
	"path/filepath"

	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/providers"
	"github.com/chiisen/mini_bot/pkg/session"
	"github.com/chiisen/mini_bot/pkg/tools"
)

type AgentInstance struct {
	Config       *config.Config
	Provider     providers.LLMProvider
	Registry     *tools.ToolRegistry
	Sessions     *session.Manager
	CtxBuilder   *Builder
	WorkspaceDir string
}

func NewInstance(cfg *config.Config) (*AgentInstance, error) {
	// 1. Get model config
	modelCfg, err := cfg.FindModel(cfg.Agents.Defaults.Model)
	if err != nil {
		return nil, fmt.Errorf("agent model config not found: %w", err)
	}

	// 2. Create LLM Provider
	provider, err := providers.NewProvider(modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// 3. Setup Workspace & Sandbox
	workspaceDir := cfg.Agents.Defaults.Workspace
	sandbox, err := tools.NewSandbox(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox initialization failed for workspace %s: %w", workspaceDir, err)
	}

	// 4. Create ToolRegistry and register basic tools
	registry := tools.NewRegistry()
	registry.Register(&tools.ReadFileTool{Sandbox: sandbox})
	registry.Register(&tools.WriteFileTool{Sandbox: sandbox})
	registry.Register(&tools.AppendFileTool{Sandbox: sandbox})
	registry.Register(&tools.ListDirTool{Sandbox: sandbox})
	registry.Register(&tools.EditFileTool{Sandbox: sandbox})
	registry.Register(&tools.ExecTool{Sandbox: sandbox})
	registry.Register(&tools.WebSearchTool{})

	// 5. Create Session Manager
	sessMgr := session.NewManager(filepath.Join(workspaceDir, "sessions"))

	// 6. Create Context Builder
	ctxBuilder := NewContextBuilder(workspaceDir)

	return &AgentInstance{
		Config:       cfg,
		Provider:     provider,
		Registry:     registry,
		Sessions:     sessMgr,
		CtxBuilder:   ctxBuilder,
		WorkspaceDir: workspaceDir,
	}, nil
}
