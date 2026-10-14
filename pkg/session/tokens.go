package session

import "github.com/chiisen/mini_bot/pkg/providers"

// DefaultContextBudgetTokens 是未設定 maxTokens 時使用的預設預算。
const DefaultContextBudgetTokens = 8192

// messageOverheadTokens 是每則訊息在 API 格式中固定要付的成本（角色、分隔符等），
// 與內容長度無關。
const messageOverheadTokens = 4

// estimateTokens 以字元估算一段文字的 token 數。
//
// 這是刻意的粗略估計：本專案維持零第三方依賴、不引入 tokenizer，而且真正的上下文
// 上限取決於供應商與模型，無法從設定得知。估計的目的是讓壓縮取決於「內容長度」而不是
// 「訊息則數」——100 則短訊息不該被丟掉，少數幾則長訊息卻可能塞不進模型。
//
// 規則：寬字元（CJK、全形標點、emoji）約 1 字元 1 token，其餘約 4 字元 1 token。
// 誤差兩成以內對這個用途足夠；它不是計費依據。
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}

	wide, narrow := 0, 0
	for _, r := range s {
		if isWideRune(r) {
			wide++
			continue
		}
		narrow++
	}

	return wide + (narrow+3)/4
}

// isWideRune 判斷是否為「一個字元約等於一個 token」的寬字元。
//
// 以 0x2E80（CJK 部首延伸區起點）為界，涵蓋漢字、假名、諺文、全形標點與 emoji。
func isWideRune(r rune) bool {
	return r >= 0x2E80
}

// estimateMessageTokens 估算單則訊息的成本，含工具呼叫的名稱與參數。
func estimateMessageTokens(msg providers.Message) int {
	cost := estimateTokens(msg.Content) + messageOverheadTokens

	for _, call := range msg.ToolCalls {
		cost += estimateTokens(call.Function.Name) + estimateTokens(call.Function.Arguments) + messageOverheadTokens
	}

	return cost
}
