# tuide build & debug targets.
# Requires Go 1.26+. Runs on Windows and Unix.

GO  ?= go
BIN ?= tuide
SRC := ./cmd/tuide

# The native binary gets the host's executable suffix, so `make build` works
# on Windows without a second rule.
EXE     := $(shell $(GO) env GOEXE)
BINPATH := $(BIN)$(EXE)

.PHONY: all build run vet test fmt fmt-fix check clean windows linux debug

all: build

build:
	$(GO) build -o $(BINPATH) $(SRC)

run:
	$(GO) run $(SRC)

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

# Cross-platform builds.
windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN).exe $(SRC)

linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BIN) $(SRC)

# Debug build with race detector and verbose test output.
debug:
	$(GO) test -race -v ./...
