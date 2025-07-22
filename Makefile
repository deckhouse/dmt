.DEFAULT_GOAL = test
.PHONY: FORCE

# enable consistent Go 1.12/1.13 GOPROXY behavior.
export GOPROXY = https://proxy.golang.org

BINARY = dmt
ifeq ($(OS),Windows_NT)
	BINARY := $(BINARY).exe
endif

# Build

build: $(BINARY)
.PHONY: build

build_race:
	go build -race -o $(BINARY) ./cmd/dmt
.PHONY: build_race

clean:
	rm -f $(BINARY)
.PHONY: clean

# Test
test:
	rm -f coverage.out
	go test -v ./... -coverprofile=coverage.out
.PHONY: test

# Linting
lint:
	golangci-lint run
.PHONY: lint

lint-fast:
	golangci-lint run --fast-only
.PHONY: lint-fast

lint-fix:
	golangci-lint run --fix
.PHONY: lint-fix

lint-fix-fast:
	golangci-lint run --fast-only --fix
.PHONY: lint-fix-fast

# Git hooks setup
setup-hooks:
	./scripts/setup-hooks.sh
.PHONY: setup-hooks

# Non-PHONY targets (real files)

$(BINARY): FORCE
	go build -o $@ ./cmd/dmt

go.mod: FORCE
	go mod tidy
	go mod verify
go.sum: go.mod

# Functions

# Check that given variables are set and all have non-empty values,
# die with an error otherwise.
#
# Params:
#   1. Variable name(s) to test.
#   2. (optional) Error message to print.
#
# https://stackoverflow.com/a/10858332/8228109
check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
      $(error Undefined $1$(if $2, ($2))))
