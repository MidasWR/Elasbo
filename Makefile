.PHONY: build test tidy download tools snapshot release release-upload ensure-release-tag

BINARY := elsabo
GO     ?= go

# Local tool binaries (not committed)
TOOLS_DIR          := $(CURDIR)/bin
GORELEASER_VERSION ?= v2.11.2
GORELEASER         := $(TOOLS_DIR)/goreleaser

build:
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o dist/$(BINARY) ./cmd/$(BINARY)

test:
	$(GO) test -count=1 ./...

tidy:
	$(GO) mod tidy

download:
	$(GO) mod download

# GoReleaser via `go install` (one-time into ./bin)
$(GORELEASER):
	@mkdir -p $(TOOLS_DIR)
	@echo "Installing GoReleaser $(GORELEASER_VERSION) -> $(GORELEASER)"
	GOBIN=$(TOOLS_DIR) $(GO) install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

tools: $(GORELEASER)

# GoReleaser (не --snapshot) требует тег v* ровно на HEAD
ensure-release-tag:
	@git describe --tags --match 'v*' --exact-match HEAD >/dev/null 2>&1 || { \
		echo "GoReleaser: на текущем коммите нет тега вида v0.1.0."; \
		echo "Создайте тег на HEAD и отправьте на origin, затем: make release-upload"; \
		echo "  git tag v0.1.0"; \
		echo "  git push origin v0.1.0"; \
		exit 1; \
	}

# Snapshot: зависимости, tidy, тесты, сборка артефактов в dist/ (без GitHub, без токена)
snapshot: $(GORELEASER) download tidy test
	$(GORELEASER) release --snapshot --clean

# Локально без токена: snapshot в dist/ (как make snapshot). С GITHUB_TOKEN в окружении — полный goreleaser release.
# Заливка релиза: make release-upload (токен берётся из GITHUB_TOKEN или из `gh auth token`, если установлен GitHub CLI).
release: $(GORELEASER) download tidy test
ifeq ($(strip $(GITHUB_TOKEN)),)
	@echo '>>> Без GITHUB_TOKEN — локальный snapshot в dist/. Публикация: gh auth login && make release-upload'
	$(GORELEASER) release --snapshot --clean
else
	@$(MAKE) --no-print-directory ensure-release-tag
	$(GORELEASER) release --clean
endif

release-upload: $(GORELEASER) download tidy test ensure-release-tag
	@set -e; \
	TOKEN="$(GITHUB_TOKEN)"; \
	if [ -z "$$TOKEN" ] && command -v gh >/dev/null 2>&1; then \
		TOKEN=$$(gh auth token 2>/dev/null || true); \
	fi; \
	if [ -z "$$TOKEN" ]; then \
		echo "Публикация релиза: задайте GITHUB_TOKEN или выполните gh auth login (нужен scope на репозиторий)."; \
		exit 1; \
	fi; \
	GITHUB_TOKEN="$$TOKEN" $(GORELEASER) release --clean
