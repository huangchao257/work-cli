VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/huangchao257/work-cli/internal/cli.Version=$(VERSION)

.PHONY: build build-all test clean package

build:
	go build -ldflags "$(LDFLAGS)" -o bin/work ./cmd/work

build-all: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/work-linux-amd64 ./cmd/work
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/work-linux-arm64 ./cmd/work
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/work-darwin-amd64 ./cmd/work
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/work-darwin-arm64 ./cmd/work
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/work-windows-amd64.exe ./cmd/work
	@cd dist && shasum -a 256 work-* > checksums.txt 2>/dev/null || sha256sum work-* > checksums.txt

package: build-all
	@echo "产物目录: dist/"
	@ls -la dist/

test:
	go test ./...

clean:
	rm -rf bin/work dist/
