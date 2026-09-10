GO ?= go
GOFMT ?= gofmt
OUTPUT ?= /tmp/alias-lens-release
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.DEFAULT_GOAL := help

.PHONY: help all fmt fmt-check test vet build diff-check check install

help:
	@printf '%s\n' \
		'Alias Lens development commands' \
		'' \
		'  make help        Show this command guide. This is also what plain make does.' \
		'  make fmt         Rewrite Go source files with gofmt.' \
		'  make fmt-check   Report unformatted Go files without changing them.' \
		'  make test        Run the Go test suite.' \
		'  make vet         Run Go static analysis.' \
		'  make build       Build alias-lens at OUTPUT.' \
		'  make diff-check  Check the Git diff for whitespace errors.' \
		'  make check       Run fmt-check, test, vet, build, and diff-check.' \
		'  make install     Test, build, and copy alias-lens to BINDIR.' \
		'' \
		'Default paths' \
		'  OUTPUT=/tmp/alias-lens-release' \
		'  BINDIR=$$HOME/.local/bin' \
		'' \
		'Examples' \
		'  make build OUTPUT=./alias-lens' \
		'  make install PREFIX=/usr/local'

all: build

fmt:
	$(GOFMT) -w *.go

fmt-check:
	@files="$$($(GOFMT) -l *.go)"; \
	if [ -n "$$files" ]; then \
		printf 'Run make fmt on these files:\n%s\n' "$$files"; \
		exit 1; \
	fi

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build -buildvcs=false -o "$(OUTPUT)" .

diff-check:
	git diff --check

check: fmt-check test vet build diff-check

install: test build
	mkdir -p "$(BINDIR)"
	install -m 0755 "$(OUTPUT)" "$(BINDIR)/alias-lens"
