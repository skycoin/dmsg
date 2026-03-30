ifeq ($(OS),Windows_NT)
	SHELL := pwsh
else
	SHELL := /bin/bash
endif

.PHONY : check lint install-linters dep tidy test test-e2e test-e2e-build test-e2e-run test-e2e-test test-e2e-stop test-e2e-clean build
.PHONY : update-dep update-skywire update-skycoin push-deps sync-upstream-develop

VERSION := $(shell git describe --always)

RFC_3339 := "+%Y-%m-%dT%H:%M:%SZ"
COMMIT := $(shell git rev-list -1 HEAD)

ifeq ($(OS),Windows_NT)
	BIN := .\bin
    BIN_DIR?=.\bin
    CMD_DIR := .\cmd
	DATE := $(shell powershell -Command date -u ${RFC_3339})
	OPTS?=powershell -Command setx GO111MODULE on;
	.DEFAULT_GOAL := help-windows
else
	BIN := ${PWD}/bin
	BIN_DIR?=./bin
	CMD_DIR := ./cmd
	DATE := $(shell date -u ${RFC_3339})
	OPTS?=GO111MODULE=on
	.DEFAULT_GOAL := help
endif

TEST_OPTS:=-v -tags no_ci -cover -timeout=5m

RACE_FLAG:=-race
GOARCH:=$(shell go env GOARCH)

ifneq (,$(findstring 64,$(GOARCH)))
    TEST_OPTS:=$(TEST_OPTS) $(RACE_FLAG)
endif

DMSG_REPO := github.com/skycoin/dmsg
SKYWIRE_UTILITIES_BASE := github.com/skycoin/skywire/pkg/skywire-utilities
BUILDINFO_PATH := $(SKYWIRE_UTILITIES_BASE)/pkg/buildinfo

BUILDINFO_VERSION := -X $(BUILDINFO_PATH).version=$(VERSION)
BUILDINFO_DATE := -X $(BUILDINFO_PATH).date=$(DATE)
BUILDINFO_COMMIT := -X $(BUILDINFO_PATH).commit=$(COMMIT)

BUILDINFO?=$(BUILDINFO_VERSION) $(BUILDINFO_DATE) $(BUILDINFO_COMMIT)

BUILD_OPTS?=-mod=vendor "-ldflags=$(BUILDINFO)"
BUILD_OPTS_DEPLOY?=-mod=vendor "-ldflags=$(BUILDINFO) -w -s"

check: lint test ## Run linters and tests

check-windows: lint test-windows ## Run linters and tests on windows

lint: ## Run linters. Use make install-linters first
	golangci-lint version
	${OPTS} golangci-lint run -c .golangci.yml ./...

vendorcheck:  ## Run vendorcheck
	GO111MODULE=off vendorcheck ./...

test: ## Run tests
	-go clean -testcache &>/dev/null
	${OPTS} go test ${TEST_OPTS} ./...

test-e2e-build: ## Build Docker images for e2e tests
	cd docker && docker compose -f docker-compose.e2e.yml build

test-e2e-run: ## Start e2e test environment
	cd docker && docker compose -f docker-compose.e2e.yml up -d
	@echo "Waiting for services to be ready..."
	sleep 15

test-e2e-test: ## Run e2e tests (requires e2e-run)
	-go clean -testcache
	go test -v -timeout=10m ./internal/e2e/...

test-e2e-stop: ## Stop e2e environment
	cd docker && docker compose -f docker-compose.e2e.yml stop

test-e2e-clean: ## Stop and remove e2e environment
	cd docker && docker compose -f docker-compose.e2e.yml down -v

test-e2e: test-e2e-build test-e2e-run test-e2e-test test-e2e-stop ## Run complete e2e test suite

test-windows: ## Run tests
	-go clean -testcache
	${OPTS} go test ${TEST_OPTS} ./...

install-linters: ## Install linters
	# GO111MODULE=off go get -u github.com/FiloSottile/vendorcheck
	# For some reason this install method is not recommended, see https://github.com/golangci/golangci-lint#install
	# However, they suggest `curl ... | bash` which we should not do
	${OPTS} go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	${OPTS} go install golang.org/x/tools/cmd/goimports@latest
	${OPTS} go install github.com/incu6us/goimports-reviser@latest

install-linters-windows: ## Install linters on windows
	${OPTS} go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	${OPTS} go install golang.org/x/tools/cmd/goimports@latest
	${OPTS} go install github.com/incu6us/goimports-reviser@latest

format: tidy ## Formats the code. Must have goimports and goimports-reviser installed (use make install-linters).
	${OPTS} goimports -w -local ${DMSG_REPO} ./pkg ./cmd ./internal ./examples
	find . -type f -name '*.go' -not -path "./.git/*" -not -path "./vendor/*"  -exec goimports-reviser -project-name ${DMSG_REPO} {} \;


format-windows: ## Formats the code. Must have goimports and goimports-reviser installed (use make install-linters-windows).
	powershell -Command .\scripts\format-windows.ps1

tidy: ## Tidies dependencies
	${OPTS} go mod tidy -v

dep: tidy ## Sorts and vendors dependencies
	${OPTS} go mod vendor -v

update-dep: ## Update all dependencies to latest versions, vendor, and commit
	${OPTS} go get -v -u ./...
	${OPTS} go mod tidy -v
	${OPTS} go mod vendor -v
	git add go.mod go.sum vendor
	git diff --cached --quiet || git commit -m "update deps"

update-skywire: ## Update skywire to latest develop branch
	@echo "Updating skywire to latest develop..."
	${OPTS} go get -v github.com/skycoin/skywire@develop
	${OPTS} go mod tidy -v
	${OPTS} go mod vendor -v
	@echo "skywire updated successfully"

update-skycoin: ## Update skycoin to latest develop branch
	@echo "Updating skycoin to latest develop..."
	${OPTS} go get -v github.com/skycoin/skycoin@develop
	${OPTS} go mod tidy -v
	${OPTS} go mod vendor -v
	@echo "skycoin updated successfully"

push-deps: ## Commit and push dependency updates
	@echo "Committing dependency updates..."
	git add go.mod go.sum vendor
	git diff --cached --quiet || git commit -m "update deps"
	git push
	@echo "Dependencies pushed successfully"

sync-upstream-develop: ## Sync local develop branch with upstream skycoin/dmsg develop
	@normalize() { \
		echo "$$1" | sed \
			-e 's|git@github.com:|https://github.com/|' \
			-e 's|ssh://github.com/|https://github.com/|' \
			-e 's|\.git$$||' \
			-e 's|https://github.com/||' \
			| tr '[:upper:]' '[:lower:]'; \
	}; \
	UPSTREAM_URL=$$(git remote get-url upstream 2>/dev/null); \
	if [ -z "$$UPSTREAM_URL" ]; then \
		echo "[error] no 'upstream' remote found. Add it with:"; \
		echo "  git remote add upstream https://github.com/skycoin/dmsg.git"; \
		exit 1; \
	fi; \
	UPSTREAM_NORM=$$(normalize "$$UPSTREAM_URL"); \
	if [ "$$UPSTREAM_NORM" != "skycoin/dmsg" ]; then \
		echo "[error] upstream remote does not point to skycoin/dmsg."; \
		echo "  Found: $$UPSTREAM_URL"; \
		exit 1; \
	fi; \
	ORIGIN_URL=$$(git remote get-url origin 2>/dev/null); \
	if [ -z "$$ORIGIN_URL" ]; then \
		echo "[error] no 'origin' remote found."; \
		exit 1; \
	fi; \
	ORIGIN_NORM=$$(normalize "$$ORIGIN_URL"); \
	if [ "$$ORIGIN_NORM" = "skycoin/dmsg" ]; then \
		echo "[error] origin points to skycoin/dmsg directly."; \
		echo "  This target must be run from a fork, not the canonical repo."; \
		exit 1; \
	fi; \
	echo "[ok] origin is a fork ($$ORIGIN_NORM), upstream is skycoin/dmsg — syncing develop..."; \
	git checkout develop && \
	git pull && \
	git fetch upstream && \
	git merge upstream/develop && \
	git push

install: ## Install `dmsg-discovery`, `dmsg-server`, `dmsgcurl`,`dmsgpty-cli`, `dmsgpty-host`, `dmsgpty-ui`
	${OPTS} go install ${BUILD_OPTS} ./cmd/*

build: ## Build binaries into ./bin
	mkdir -p ${BIN}; go build ${BUILD_OPTS} -o ${BIN} ${CMD_DIR}/*

build-windows: ## Build binaries into ./bin
	powershell -Command new-item ${BIN} -itemtype directory -force
	powershell 'Get-ChildItem ${CMD_DIR} | % { go build ${BUILD_OPTS} -o ${BIN} $$_.FullName }'

build-deploy: ## Build for deployment Docker images
	go build -tags netgo ${BUILD_OPTS_DEPLOY} -o /release/dmsg-discovery ./cmd/dmsg-discovery
	go build -tags netgo ${BUILD_OPTS_DEPLOY} -o /release/dmsg-server ./cmd/dmsg-server

build-docker:
	./docker/scripts/docker-push.sh -t "develop" -b

github-prepare-release:
	$(eval GITHUB_TAG=$(shell git describe --abbrev=0 --tags | cut -c 2-6))
	sed '/^## ${GITHUB_TAG}$$/,/^## .*/!d;//d;/^$$/d' ./CHANGELOG.md > releaseChangelog.md

github-release: github-prepare-release
	goreleaser --clean --config .goreleaser-linux.yml --release-notes releaseChangelog.md

github-release-darwin:
	goreleaser --clean  --config .goreleaser-darwin.yml  --skip=publish
	$(eval GITHUB_TAG=$(shell git describe --abbrev=0 --tags))
	gh release upload --repo skycoin/dmsg ${GITHUB_TAG} ./dist/dmsg-${GITHUB_TAG}-darwin-amd64.tar.gz
	gh release upload --repo skycoin/dmsg ${GITHUB_TAG} ./dist/dmsg-${GITHUB_TAG}-darwin-arm64.tar.gz
	gh release download ${GITHUB_TAG} --repo skycoin/dmsg --pattern 'checksums*'
	cat ./dist/checksums.txt >> ./checksums.txt
	gh release upload --repo skycoin/dmsg ${GITHUB_TAG} --clobber ./checksums.txt

github-release-windows:
	.\goreleaser\goreleaser.exe --clean  --config .goreleaser-windows.yml --skip=publish
	$(eval GITHUB_TAG=$(shell powershell git describe --abbrev=0 --tags))
	gh release upload --repo skycoin/dmsg ${GITHUB_TAG} ./dist/dmsg-${GITHUB_TAG}-windows-amd64.zip
	gh release upload --repo skycoin/dmsg ${GITHUB_TAG} ./dist/dmsg-${GITHUB_TAG}-windows-386.zip
	gh release download ${GITHUB_TAG} --repo skycoin/dmsg --pattern 'checksums*'
	cat ./dist/checksums.txt >> ./checksums.txt
	gh release upload --repo skycoin/dmsg ${GITHUB_TAG} --clobber ./checksums.txt

dep-github-release:
	mkdir musl-data
	wget -c https://more.musl.cc/10/x86_64-linux-musl/aarch64-linux-musl-cross.tgz -O aarch64-linux-musl-cross.tgz
	tar -xzf aarch64-linux-musl-cross.tgz -C ./musl-data && rm aarch64-linux-musl-cross.tgz
	wget -c https://more.musl.cc/10/x86_64-linux-musl/arm-linux-musleabi-cross.tgz -O arm-linux-musleabi-cross.tgz
	tar -xzf arm-linux-musleabi-cross.tgz -C ./musl-data && rm arm-linux-musleabi-cross.tgz
	wget -c https://more.musl.cc/10/x86_64-linux-musl/arm-linux-musleabihf-cross.tgz -O arm-linux-musleabihf-cross.tgz
	tar -xzf arm-linux-musleabihf-cross.tgz -C ./musl-data && rm arm-linux-musleabihf-cross.tgz
	wget -c https://more.musl.cc/10/x86_64-linux-musl/x86_64-linux-musl-cross.tgz -O x86_64-linux-musl-cross.tgz
	tar -xzf x86_64-linux-musl-cross.tgz -C ./musl-data && rm x86_64-linux-musl-cross.tgz

snapshot-linux: snapshot-clean
	goreleaser --snapshot --config .goreleaser-linux.yml --skip-publish --rm-dist

snapshot-darwin: snapshot-clean
	goreleaser --snapshot --config .goreleaser-darwin.yml --skip-publish --rm-dist

snapshot-windows: snapshot-clean
	goreleaser --snapshot --config .goreleaser-windows.yml --skip-publish --rm-dist

snapshot-clean: ## Cleans snapshot / release
	rm -rf ./dist

start-db: ## Init local database env.
	source ./integration/env.sh && init_redis

stop-db: ## Stop local database env.
	source ./integration/env.sh && stop_redis

attach-db: ## Attach local database env.
	source ./integration/env.sh && attach_redis

start-dmsg: build ## Init local dmsg env.
	source ./integration/env.sh && init_dmsg

stop-dmsg: ## Stop local dmsg env.
	source ./integration/env.sh && stop_dmsg

attach-dmsg: ## Attach local dmsg tmux session.
	source ./integration/env.sh && attach_dmsg

start-pty: build ## Init local dmsgpty env.
	source ./integration/env.sh && init_dmsgpty

stop-pty: ## Stop local dmsgpty env.
	source ./integration/env.sh && stop_dmsgpty

attach-pty: ## Attach local dmsgpty tmux session.
	source ./integration/env.sh && attach_dmsgpty

stop-all: stop-pty stop-dmsg stop-db ## Stop all local tmux sessions.

integration-windows-start: ## Start integration test on windows.
	powershell -Command .\integration\integration.ps1 start

integration-windows-stop: ## Stops integration test on windows.
	powershell -Command .\integration\integration.ps1 stop

help: ## Display help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

help-windows: ## Display help for windows
	@powershell 'Select-String -Pattern "windows[a-zA-Z_-]*:.*## .*$$" $(MAKEFILE_LIST) | % { $$_.Line -split ":.*?## " -Join "`t:`t" } '
