GO ?= go
GOFMT ?= gofmt
OUTPUT ?= /tmp/alias-lens-release
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: all fmt fmt-check test vet build diff-check check install

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

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 "$(OUTPUT)" "$(BINDIR)/alias-lens"
