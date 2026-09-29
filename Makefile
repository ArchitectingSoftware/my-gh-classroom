BINARY  := mgc
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/ArchitectingSoftware/my-gh-classroom/cmd.version=$(VERSION)

.PHONY: build install clean test vet fmt tidy release version

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install:
	go build -ldflags "$(LDFLAGS)" -o $$(go env GOPATH)/bin/$(BINARY) .

version:
	@echo $(VERSION)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w main.go cmd internal

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
	rm -rf dist

release: tidy
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$(VERSION)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$(VERSION)-darwin-amd64 .
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$(VERSION)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$(VERSION)-linux-arm64 .
