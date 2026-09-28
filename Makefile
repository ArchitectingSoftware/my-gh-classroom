BINARY := mgc

.PHONY: build install clean test vet fmt tidy release

build:
	go build -o $(BINARY) .

install:
	go build -o $$(go env GOPATH)/bin/$(BINARY) .

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
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64 .
	GOOS=linux GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64 .
