VERSION ?= 0.1.0
LDFLAGS := -X github.com/setupproof/setupproof/internal/app.Version=$(VERSION)

.PHONY: build test vet race fmt fmt-check dogfood foundation action docs examples launch-polish check staticcheck vuln actionlint release-archives

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

launch-polish:
	sh scripts/check-launch-polish.sh

check: test vet race foundation action docs examples launch-polish dogfood

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

actionlint:
	go run github.com/rhysd/actionlint/cmd/actionlint@latest .github/workflows/setupproof.yml

release-archives:
	scripts/package-release.sh v$(VERSION)
