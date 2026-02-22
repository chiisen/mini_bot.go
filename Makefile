.PHONY: build run clean test lint size release

APP_NAME = app
CMD_DIR = ./cmd/appname/

build:
	go build -ldflags="-s -w" -o $(APP_NAME) $(CMD_DIR)

run: build
	./$(APP_NAME) $(ARGS)

clean:
	rm -f $(APP_NAME)
	rm -rf dist/

test:
	go test ./...

lint:
	go vet ./...

size: build
	ls -lh $(APP_NAME)

release:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/minibot-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/minibot-linux-arm64 $(CMD_DIR)
	GOOS=linux GOARCH=riscv64 go build -ldflags="-s -w" -o dist/minibot-linux-riscv64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/minibot-darwin-arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/minibot-windows-amd64.exe $(CMD_DIR)
