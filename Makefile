.DEFAULT_GOAL = test
.PHONY: FORCE

# enable consistent Go 1.12/1.13 GOPROXY behavior.
export GOPROXY = https://proxy.golang.org

BINARY = d8-lint
ifeq ($(OS),Windows_NT)
	BINARY := $(BINARY).exe
endif

# Build

build: $(BINARY)
.PHONY: build

build_race:
	go build -race -o $(BINARY) ./cmd/d8-lint
.PHONY: build_race

clean:
	rm -f $(BINARY)
.PHONY: clean

# Test
test:
	go test -v -parallel 2 ./...
.PHONY: test

# Non-PHONY targets (real files)

$(BINARY): FORCE
	go build -o $@ ./cmd/d8-lint

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
