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

# Install the translation files next to the binary. 程式碼的兩條尋找路徑
# （pkg/config.Load 與 pkg/i18n 的惰性初始化）都以執行檔所在目錄為優先，
# 因此只需要放一份，而且不受 `-w` 指定的工作目錄影響。
# 少了這些檔案，容器內所有 i18n 字串都會退化成原始 key（例如
# `minibot version` 印出 "app.name app.version"）。
COPY --from=builder /src/lang /usr/local/bin/lang

# Setup entrypoint dir
WORKDIR /app
ENTRYPOINT ["minibot"]
CMD ["gateway"]
