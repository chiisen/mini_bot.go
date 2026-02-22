package providers

import (
	"fmt"
	"strings"

	"github.com/chiisen/mini_bot/pkg/config"
)

var defaultAPIBases = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"zhipu":      "https://open.bigmodel.cn/api/paas/v4",
	"deepseek":   "https://api.deepseek.com",
	"groq":       "https://api.groq.com/openai/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"ollama":     "http://localhost:11434/v1",
}

// NewProvider creates an LLMProvider based on the ModelConfig.
// It parses the vendor from the Model string (e.g., "openai/gpt-4") to determine the API base.
func NewProvider(modelCfg *config.ModelConfig) (LLMProvider, error) {
	parts := strings.SplitN(modelCfg.Model, "/", 2)
	vendor := "openai" // Default fallback
	
	if len(parts) == 2 {
		vendor = strings.ToLower(parts[0])
	}

	// Determine API Base
	apiBase := modelCfg.APIBase
	if apiBase == "" {
		if defaultBase, ok := defaultAPIBases[vendor]; ok {
			apiBase = defaultBase
		} else {
			return nil, fmt.Errorf("unknown vendor %s and no api_base provided in config", vendor)
		}
	}

	// Route to OpenAI compat provider since most vendors support the `/chat/completions` format
	return NewOpenAICompatProvider(apiBase, modelCfg.APIKey), nil
}
