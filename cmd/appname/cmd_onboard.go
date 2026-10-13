package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultIdentity = `# 名稱與基本設定
你的名字是「MiniBot.go」，是一個運作在本地端、極致輕便、專注於協助使用者完成技術與日常任務的 AI 助理。

# 核心目標
1. 解決使用者的問題，不論是回答技術疑問或協調操作。
2. 保持低資源使用率，提供高效且有價值的回應。
3. 謹慎操作系統工具，時刻注意安全性與範圍限制。`

const defaultAgent = `# 行為指引
1. **分析優先**：在執行任何指令或給出代碼前，先了解使用者的確切目標和上下文。
2. **善用工具**：當需要讀寫檔案、取得目錄結構或執行指令時，主動使用系統提供的工具。
3. **安全第一**：切勿執行具有破壞性或修改系統全域配置的危險指令。任何超出 workspace 的操作必須果斷拒絕。
4. **精準回答**：避免給出沒有意義的資訊，專注於解決當前的問題。`

const defaultSoul = `# 個性特質
- **簡潔高效**：你不喜歡長篇大論，回答總是切中要害。
- **冷靜誠實**：不知道的資訊就回答不知道，不瞎扯。
- **友善平易**：在簡潔的同時，會保持溫和且合作的態度。`

const defaultUser = `# 使用者偏好
語言：預設使用繁體中文。
時區：Asia/Taipei。
風格偏好：技術問題請提供帶有註解的完整程式碼或明確且可被直接複製執行的指令。`

const defaultConfig = `{
  "agents": {
    "defaults": {
      "workspace": "~/.minibot.go/workspace",
      "model": "gpt4",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20,
      "restrict_to_workspace": true
    }
  },
  "model_list": [
    {
      "model_name": "gpt4",
      "model": "openai/gpt-4",
      "api_key": "YOUR_API_KEY_HERE"
    },
    {
      "model_name": "llama3",
      "model": "ollama/llama3",
      "api_base": "http://localhost:11434/v1",
      "api_key": "ollama"
    }
  ],
  "channels": {
    "telegram": {
      "enabled": false,
      "token": "YOUR_BOT_TOKEN_HERE",
      "allow_from": ["YOUR_TELEGRAM_USER_ID"]
    }
  }
}`

// RunOnboard handles the 'app onboard' command.
func RunOnboard(args []string) error {
	fmt.Println("🌟 Initializing MiniBot.go workspace...")

	configDir := expandHome("~/.minibot.go")
	workspaceDir := filepath.Join(configDir, "workspace")

	// Prompt for default API Key
	fmt.Print("Enter your OpenAI/DeepSeek API Key (press Enter to skip): ")
	scanner := bufio.NewScanner(os.Stdin)
	apiKey := "YOUR_API_KEY_HERE"
	if scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			apiKey = text
		}
	}

	// Create directories
	dirs := []string{
		configDir,
		workspaceDir,
		filepath.Join(workspaceDir, "sessions"),
		filepath.Join(workspaceDir, "memory"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Update the config template with the provided token
	finalConfig := strings.Replace(defaultConfig, "YOUR_API_KEY_HERE", apiKey, 1)

	// Create Config File
	// 權限 0600：config.json 內含 API 金鑰，與 config.checkFilePermissions
	// 的警告條件（mode & 0077 != 0）一致，不應讓同機其他使用者讀取
	configPath := filepath.Join(configDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(finalConfig), 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", configPath, err)
		}
		fmt.Printf("✅ Created config file at: %s\n", configPath)
	} else {
		fmt.Printf("ℹ️ Config file already exists at: %s, skipped.\n", configPath)
	}

	// Create Workspace Files
	files := map[string]string{
		"IDENTITY.md": defaultIdentity,
		"AGENT.md":    defaultAgent,
		"SOUL.md":     defaultSoul,
		"USER.md":     defaultUser,
	}

	for name, content := range files {
		path := filepath.Join(workspaceDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}
			fmt.Printf("✅ Created %s\n", path)
		}
	}

	fmt.Println("🚀 Onboard successful! You can now use 'app agent' to test the assistant.")
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
