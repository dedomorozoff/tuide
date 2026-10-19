# tuide build & debug targets.
# Requires Go 1.26+. Runs on Windows and Unix.

# A POSIX shell runs the recipes even on Windows, where make would otherwise
# hand them to cmd.exe: the cross-compile targets set GOOS inline and clean
# calls rm. Git for Windows ships sh, so this holds there too.
SHELL := sh

GO  ?= go
BIN ?= tuide
SRC := ./cmd/tuide

# The native binary gets the host's executable suffix, so `make build` works
# on Windows without a second rule.
EXE     := $(shell $(GO) env GOEXE)
BINPATH := $(BIN)$(EXE)

# Printed by the help target. Kept as text rather than scraped from comments:
# scraping needs a shell tool, and make hands recipes to cmd.exe on Windows.
define HELP_TEXT
Usage: make <target>

Targets:
  build     Compile the binary for this host.
  install   Install the binary with go install.
  run       Run the desktop in this terminal.
  ls        Show the running session.
  kill      End the running session for everyone.
  vet       Run go vet.
  test      Run the tests.
  fmt       List the files gofmt would change.
  fmt-fix   Rewrite the files gofmt would change.
  check     Run vet and the tests.
  clean     Drop the build cache and the binary.
  windows   Cross-compile for windows/amd64.
  linux     Cross-compile for linux/amd64.
  debug     Run the tests with the race detector, verbose.
  help      Show this help.
endef

.PHONY: all build install run ls kill vet test fmt fmt-fix check clean windows linux debug help

all: build

build:
	$(GO) build -o $(BINPATH) $(SRC)

install:
	$(GO) install $(SRC)

run:
	$(GO) run $(SRC)

ls:
	$(GO) run $(SRC) ls

kill:
	$(GO) run $(SRC) kill-server

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

fmt:
	gofmt -l .

fmt-fix:
	gofmt -w .

check: vet test

clean:
	$(GO) clean -cache
	rm -f $(BIN) $(BIN).exe

windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN).exe $(SRC)

linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN) $(SRC)

debug:
	$(GO) test -race -v ./...

help:
	$(info $(HELP_TEXT))
	@:
