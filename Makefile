VERSION ?= 0.1.0
LDFLAGS := -X github.com/setupproof/setupproof/internal/app.Version=$(VERSION)
STATICCHECK_VERSION ?= v0.6.1
GOVULNCHECK_VERSION ?= v1.1.4
ACTIONLINT_VERSION ?= v1.7.7

.PHONY: build test vet race fmt fmt-check dogfood foundation action docs examples check staticcheck vuln actionlint release-archives release-check

build:
	go build -ldflags "$(LDFLAGS)" -o ./setupproof ./cmd/setupproof

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

fmt-check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))"

dogfood: build
	./setupproof --include-untracked --require-blocks --no-color --no-glyphs README.md

foundation:
	sh scripts/check-foundation.sh

action:
	bash scripts/check-github-action.sh

docs:
	sh scripts/check-docs.sh

examples:
	sh scripts/check-examples.sh

check: test vet race foundation action docs examples dogfood

staticcheck:
	GOTOOLCHAIN=auto go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

vuln:
	GOTOOLCHAIN=auto go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

actionlint:
	GOTOOLCHAIN=auto go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION) .github/workflows/setupproof.yml .github/workflows/release-checks.yml

release-archives:
	scripts/package-release.sh v$(VERSION)

release-check: release-archives
	scripts/check-release-archives.sh v$(VERSION)
