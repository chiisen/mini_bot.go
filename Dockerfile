# Builder Stage
# 基底版本需與 go.mod 的 go 指令一致（目前為 1.25）
FROM golang:1.25-alpine AS builder

# 只使用映像檔自帶的 Go 版本：若與 go.mod 需求不符就直接建置失敗，
# 而不是靜默下載另一個 toolchain 蓋掉版本不一致的問題
ENV GOTOOLCHAIN=local

WORKDIR /src
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/minibot ./cmd/appname

# Final minimalistic Stage
FROM alpine:latest

# Install basic CA certificates and curl since it might do web searches
RUN apk --no-cache add ca-certificates curl bash

# Install the binary
COPY --from=builder /bin/minibot /usr/local/bin/minibot

# Install the translation files. 程式碼裡有兩條不同的尋找路徑，在預設的 WORKDIR
# 下兩者都要滿足：
#   - pkg/config 的 Load 以執行檔所在目錄為優先，故複製到 /usr/local/bin/lang
#   - pkg/i18n 的惰性初始化只看當前工作目錄，故複製到 /app/lang（= WORKDIR）
# 註：若以 -w 指定其他工作目錄，version / help 這類不載入 config 的指令仍會
# 找不到語言檔（pkg/i18n 未像 config.Load 一樣檢查執行檔所在目錄）。
# 少了這些檔案，容器內所有 i18n 字串都會退化成原始 key，例如
# `minibot version` 會印出 "app.name app.version"，系統提示詞的段落標題
# 則變成 "sections.identity"。
COPY --from=builder /src/lang /usr/local/bin/lang
COPY --from=builder /src/lang /app/lang

# Setup entrypoint dir
WORKDIR /app
ENTRYPOINT ["minibot"]
CMD ["gateway"]
