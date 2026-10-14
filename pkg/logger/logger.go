package logger

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// redactPattern 是一個敏感樣式及其對應的遮蔽字串。
//
// 替換字串依樣式是否帶捕捉群組而定：帶群組的保留鍵名（`api_key=**[REDACTED]**`），
// 不帶群組的整段替換。先前所有樣式共用 "$1=**[REDACTED]**"，但其中兩個樣式沒有
// 捕捉群組，$1 會展開成空字串——遮蔽結果雖然正確，語意卻是錯的。
type redactPattern struct {
	re          *regexp.Regexp
	replacement string
}

var (
	defaultLogger *slog.Logger

	sensitiveKeys = []string{
		"api_key", "apikey", "api-key",
		"token", "access_token", "access-token",
		"password", "passwd", "secret",
		"bot_token", "bot-token",
		"private_key", "private-key",
	}

	// 順序有意義：**特定樣式必須排在通用樣式之前**。若先套用通用的 key=value
	// 樣式，它會先吃掉 `bot_token=123456789` 的數字前綴，讓 bot token 的專用樣式
	// 再也比對不到，結果 `:AAF-...` 這段密碼會留在日誌裡（由 logger_test.go 抓到）。
	redactPatterns = []redactPattern{
		{
			regexp.MustCompile(`(?i)bot[_-]?token["']?\s*[:=]\s*["']?\d{8,}:[a-zA-Z0-9_\-]{20,}`),
			"**[REDACTED]**",
		},
		{
			regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{20,}`),
			"**[REDACTED]**",
		},
		{
			regexp.MustCompile(`(?i)(api[_-]?key|token|password|secret)["']?\s*[:=]\s*["']?([a-zA-Z0-9_\-\.]{8,})["']?`),
			"$1=**[REDACTED]**",
		},
	}
)

func sanitizeArgs(args ...any) []any {
	var sanitized []any
	for i := 0; i < len(args); i++ {
		key, ok := args[i].(string)
		if !ok {
			sanitized = append(sanitized, args[i])
			continue
		}
		keyLower := strings.ToLower(key)
		isSensitive := false
		for _, sk := range sensitiveKeys {
			if strings.Contains(keyLower, sk) {
				isSensitive = true
				break
			}
		}
		sanitized = append(sanitized, key)
		if i+1 < len(args) {
			if isSensitive {
				sanitized = append(sanitized, "[REDACTED]")
			} else {
				sanitized = append(sanitized, args[i+1])
			}
			i++
		}
	}
	return sanitized
}

func sanitizeMessage(msg string) string {
	for _, pattern := range redactPatterns {
		msg = pattern.re.ReplaceAllString(msg, pattern.replacement)
	}
	return msg
}

func Init(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func Debug(msg string, args ...any) {
	slog.Debug(sanitizeMessage(msg), sanitizeArgs(args...)...)
}

func Info(msg string, args ...any) {
	slog.Info(sanitizeMessage(msg), sanitizeArgs(args...)...)
}

func Warn(msg string, args ...any) {
	slog.Warn(sanitizeMessage(msg), sanitizeArgs(args...)...)
}

func Error(msg string, args ...any) {
	slog.Error(sanitizeMessage(msg), sanitizeArgs(args...)...)
}
