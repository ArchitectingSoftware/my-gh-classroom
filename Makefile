BINARY  := mgc
MODULE  := github.com/ArchitectingSoftware/my-gh-classroom
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X $(MODULE)/cmd.version=$(VERSION)

# Release builds: static, reproducible-ish, no local paths, stripped.
DIST          := dist
PLATFORMS     := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64
REL_LDFLAGS   := -s -w $(LDFLAGS)
ARCHIVE_VER   := $(VERSION:v%=%)
CHECKSUMS     := $(DIST)/$(BINARY)_$(ARCHIVE_VER)_checksums.txt
# Release tags are semver with a leading v: v1.2.3 or v1.2.3-rc.1
TAG_RE        := ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$

.DEFAULT_GOAL := build
.PHONY: build install clean test vet fmt tidy version \
        check-release dist release publish

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
	rm -rf $(DIST)

# ---------------------------------------------------------------- releases
#
#   make dist      cross-compile archives + checksums for ANY version
#                  (snapshot builds for testing; nothing is published)
#   make release   same, but only from a clean checkout of a vX.Y.Z tag,
#                  after vet and tests pass
#   make publish   make release, then create the GitHub release and upload
#                  dist/ (the tag must already be pushed to origin)

# check-release refuses to build a release from anything but a clean,
# exactly-tagged commit with a semver tag.
check-release:
	@tags=$$(git tag --points-at HEAD); \
	if [ -z "$$tags" ]; then \
	  echo "error: HEAD is not tagged. Tag it first, e.g.:"; \
	  echo "  git tag v1.0.0 && git push origin v1.0.0"; exit 1; fi; \
	tag=$(VERSION); \
	echo "$$tags" | grep -qx "$$tag" || { \
	  echo "error: VERSION=$$tag is not a tag on HEAD (HEAD has: $$(echo $$tags))"; \
	  echo "  use: make release VERSION=<one of those tags>"; exit 1; }; \
	echo "$$tag" | grep -Eq '$(TAG_RE)' || { \
	  echo "error: tag $$tag is not a release tag (expected vX.Y.Z or vX.Y.Z-prerelease)"; exit 1; }; \
	if [ -n "$$(git status --porcelain)" ]; then \
	  echo "error: working tree is not clean (uncommitted or untracked files); commit, stash, or gitignore them"; \
	  git status --short; exit 1; fi; \
	if ! out=$$(go mod tidy -diff 2>&1); then \
	  echo "error: go.mod/go.sum are not tidy (run 'make tidy' and commit):"; \
	  echo "$$out" | head -20; exit 1; fi; \
	echo "release $$tag: clean tree, tag OK"

dist:
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@set -e; for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; ext=""; [ $$os = windows ] && ext=.exe; \
	  name=$(BINARY)_$(ARCHIVE_VER)_$${os}_$${arch}; \
	  stage=$(DIST)/.stage/$$name; mkdir -p $$stage; \
	  echo "build   $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath \
	    -ldflags "$(REL_LDFLAGS)" -o $$stage/$(BINARY)$$ext .; \
	  cp README.md GETTING_STARTED.md LICENSE $$stage/; \
	  if [ $$os = windows ]; then \
	    (cd $(DIST)/.stage && zip -qr ../$$name.zip $$name); \
	  else \
	    tar -C $(DIST)/.stage -czf $(DIST)/$$name.tar.gz $$name; \
	  fi; \
	done
	@rm -rf $(DIST)/.stage
	@cd $(DIST) && if command -v sha256sum >/dev/null 2>&1; then \
	  sha256sum *.tar.gz *.zip; else shasum -a 256 *.tar.gz *.zip; fi \
	  > $(notdir $(CHECKSUMS))
	@echo; echo "Artifacts in $(DIST)/ for $(VERSION):"; ls -1 $(DIST)

release: check-release vet test dist

publish: release
	@command -v gh >/dev/null || { echo "error: gh CLI not found"; exit 1; }
	@if gh release view $(VERSION) >/dev/null 2>&1; then \
	  echo "error: GitHub release $(VERSION) already exists"; exit 1; fi
	gh release create $(VERSION) $(DIST)/*.tar.gz $(DIST)/*.zip $(CHECKSUMS) \
	  --verify-tag --title "$(BINARY) $(VERSION)" --generate-notes \
	  $(if $(findstring -,$(VERSION)),--prerelease,)
