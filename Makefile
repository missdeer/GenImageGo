.PHONY: all clean

# Detect OS and architecture
UNAME_S := $(shell uname -s 2>/dev/null || echo Windows)

ifeq ($(UNAME_S),Darwin)
    EXT :=
    IS_MACOS := 1
else ifeq ($(OS),Windows_NT)
    EXT := .exe
    IS_MACOS :=
else
    EXT :=
    IS_MACOS :=
endif

# Build flags
LDFLAGS := -ldflags="-s -w"

# Output directory
BINDIR := bin

# Targets
MAIN := $(BINDIR)/genImage$(EXT)
SEED := $(BINDIR)/seed$(EXT)
ADDUSER := $(BINDIR)/adduser$(EXT)

# Source files
MAIN_SRCS := $(wildcard *.go) $(wildcard */*.go) go.mod go.sum
SEED_SRCS := $(wildcard cmd/seed/*.go) $(wildcard model/*.go) go.mod go.sum
ADDUSER_SRCS := $(wildcard cmd/adduser/*.go) $(wildcard model/*.go) go.mod go.sum

all: $(MAIN) $(SEED) $(ADDUSER)

ifdef IS_MACOS
# macOS: Build FAT binary (Universal Binary) for arm64 and amd64
$(MAIN): $(MAIN_SRCS)
	@mkdir -p $(BINDIR)
	GOARCH=arm64 go build $(LDFLAGS) -o $@_arm64 .
	GOARCH=amd64 go build $(LDFLAGS) -o $@_amd64 .
	lipo -create -output $@ $@_arm64 $@_amd64
	@rm -f $@_arm64 $@_amd64

$(SEED): $(SEED_SRCS)
	@mkdir -p $(BINDIR)
	GOARCH=arm64 go build $(LDFLAGS) -o $@_arm64 ./cmd/seed
	GOARCH=amd64 go build $(LDFLAGS) -o $@_amd64 ./cmd/seed
	lipo -create -output $@ $@_arm64 $@_amd64
	@rm -f $@_arm64 $@_amd64

$(ADDUSER): $(ADDUSER_SRCS)
	@mkdir -p $(BINDIR)
	GOARCH=arm64 go build $(LDFLAGS) -o $@_arm64 ./cmd/adduser
	GOARCH=amd64 go build $(LDFLAGS) -o $@_amd64 ./cmd/adduser
	lipo -create -output $@ $@_arm64 $@_amd64
	@rm -f $@_arm64 $@_amd64
else
# Windows/Linux: Build single architecture
$(MAIN): $(MAIN_SRCS)
	@mkdir -p $(BINDIR)
	go build $(LDFLAGS) -o $@ .

$(SEED): $(SEED_SRCS)
	@mkdir -p $(BINDIR)
	go build $(LDFLAGS) -o $@ ./cmd/seed

$(ADDUSER): $(ADDUSER_SRCS)
	@mkdir -p $(BINDIR)
	go build $(LDFLAGS) -o $@ ./cmd/adduser
endif

clean:
	rm -rf $(BINDIR)
