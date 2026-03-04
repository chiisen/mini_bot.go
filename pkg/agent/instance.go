package agent

// ============================================================================
// Agent 實例 (Agent Instance)
// ============================================================================
// 本檔案定義了 AgentInstance 結構體，這是整個 Agent 系統的核心元件。
// AgentInstance 負責整合所有必要的元件：
//   - 配置管理 (Config)
//   - LLM 提供者 (Provider)
//   - 工具註冊表 (Registry)
//   - 對話會話管理 (Sessions)
//   - 上下文建構器 (CtxBuilder)
//   - 工作區目錄 (WorkspaceDir)
//
// 透過 NewInstance 函數，可以建立一個完整的 Agent 實例，
// 準備好處理使用者的請求。
// ============================================================================

import (
	"fmt"
	"path/filepath"

	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/providers"
	"github.com/chiisen/mini_bot/pkg/session"
	"github.com/chiisen/mini_bot/pkg/tools"
)

// ============================================================================
// AgentInstance: Agent 實例結構體
// ============================================================================
// 這是 Agent 的核心資料結構，包含了運行 Agent 所需的所有元件。
// 每個欄位都代表一個特定的職責：
//   - Config:       應用程式配置
//   - Provider:     LLM 提供者 (如 OpenAI、Claude 等)
//   - Registry:     工具註冊表，管理所有可用的工具
//   - Sessions:     對話會話管理器，負責歷史記錄的持久化
//   - CtxBuilder:   上下文建構器，用於生成系統提示詞
//   - WorkspaceDir: 工作區目錄路徑
//
// ============================================================================
type AgentInstance struct {
	Config       *config.Config        // 應用程式配置
	Provider     providers.LLMProvider // LLM 提供者介面
	Registry     *tools.ToolRegistry   // 工具註冊表
	Sessions     *session.Manager      // 對話會話管理器
	CtxBuilder   *Builder              // 上下文建構器
	WorkspaceDir string                // 工作區目錄路徑
}

// ============================================================================
// NewInstance: 建立新的 Agent 實例
// ============================================================================
// 這個函數負責初始化一個完整的 Agent 實例。
// 它會按照依賴順序逐步建立各個元件：
//  1. 取得模型配置
//  2. 建立 LLM 提供者
//  3. 初始化沙盒環境
//  4. 建立工具註冊表並註冊工具
//  5. 建立對話會話管理器
//  6. 建立上下文建構器
//
// 參數：
//   - cfg: 應用程式配置指標
//
// 回傳：
//   - *AgentInstance: 初始化完成的 Agent 實例
//   - error: 如果初始化過程中發生錯誤
//
// 錯誤可能來自：
//   - 模型配置不存在
//   - LLM 提供者建立失敗
//   - 沙盒環境初始化失敗
//
// ============================================================================
func NewInstance(cfg *config.Config) (*AgentInstance, error) {
	// -------------------------------------------------------------------------
	// 步驟 1: 取得模型配置
	// -------------------------------------------------------------------------
	// 從配置中查找預設模型的配置資訊
	// 模型配置包含 API 金鑰、端點等資訊
	modelCfg, err := cfg.FindModel(cfg.Agents.Defaults.Model)
	if err != nil {
		return nil, fmt.Errorf("agent model config not found: %w", err)
	}

	// -------------------------------------------------------------------------
	// 步驟 2: 建立 LLM 提供者
	// -------------------------------------------------------------------------
	// 根據模型配置建立對應的 LLM 提供者
	// 提供者負責與外部 LLM API 進行通信
	provider, err := providers.NewProvider(modelCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// -------------------------------------------------------------------------
	// 步驟 3: 設定工作區與沙盒環境
	// -------------------------------------------------------------------------
	// 取得工作區目錄路徑
	workspaceDir := cfg.Agents.Defaults.Workspace

	// 建立沙盒環境
	// 沙盒確保工具只能在工作區目錄內操作，防止目錄穿越攻擊
	sandbox, err := tools.NewSandbox(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("sandbox initialization failed for workspace %s: %w", workspaceDir, err)
	}

	// -------------------------------------------------------------------------
	// 步驟 4: 建立工具註冊表並註冊工具
	// -------------------------------------------------------------------------
	// 建立空的工具註冊表
	registry := tools.NewRegistry()

	// 註冊檔案操作工具
	registry.Register(&tools.ReadFileTool{Sandbox: sandbox})   // 讀取檔案
	registry.Register(&tools.WriteFileTool{Sandbox: sandbox})  // 寫入檔案
	registry.Register(&tools.AppendFileTool{Sandbox: sandbox}) // 追加檔案
	registry.Register(&tools.ListDirTool{Sandbox: sandbox})    // 列出目錄
	registry.Register(&tools.EditFileTool{Sandbox: sandbox})   // 編輯檔案

	// 註冊命令執行工具
	registry.Register(&tools.ExecTool{Sandbox: sandbox}) // 執行 Shell 命令

	// 註冊網路工具
	registry.Register(&tools.WebSearchTool{}) // 網路搜尋

	// -------------------------------------------------------------------------
	// 步驟 5: 建立對話會話管理器
	// -------------------------------------------------------------------------
	// 會話管理器負責歷史記錄的持久化
	// 對話歷史會保存在 workspace/sessions 目錄下的 JSON 檔案中
	sessMgr := session.NewManager(filepath.Join(workspaceDir, "sessions"))

	// -------------------------------------------------------------------------
	// 步驟 6: 建立上下文建構器
	// -------------------------------------------------------------------------
	// 上下文建構器用於根據工作區設定檔生成系統提示詞
	ctxBuilder := NewContextBuilder(workspaceDir)

	// -------------------------------------------------------------------------
	// 完成: 返回初始化完成的 Agent 實例
	// -------------------------------------------------------------------------
	return &AgentInstance{
		Config:       cfg,          // 應用程式配置
		Provider:     provider,     // LLM 提供者
		Registry:     registry,     // 工具註冊表
		Sessions:     sessMgr,      // 對話會話管理器
		CtxBuilder:   ctxBuilder,   // 上下文建構器
		WorkspaceDir: workspaceDir, // 工作區目錄
	}, nil
}
