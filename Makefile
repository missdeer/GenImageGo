.PHONY: all clean main seed adduser

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

# Output directory
BINDIR := bin

# Targets
MAIN := $(BINDIR)/genImage$(EXT)
SEED := $(BINDIR)/seed$(EXT)
ADDUSER := $(BINDIR)/adduser$(EXT)

all: main seed adduser

main: $(MAIN)

seed: $(SEED)

adduser: $(ADDUSER)

ifdef IS_MACOS
# macOS: Build FAT binary (Universal Binary) for arm64 and amd64
$(MAIN):
	@mkdir -p $(BINDIR)
	GOARCH=arm64 go build -o $@_arm64 .
	GOARCH=amd64 go build -o $@_amd64 .
	lipo -create -output $@ $@_arm64 $@_amd64
	@rm -f $@_arm64 $@_amd64

$(SEED):
	@mkdir -p $(BINDIR)
	GOARCH=arm64 go build -o $@_arm64 ./cmd/seed
	GOARCH=amd64 go build -o $@_amd64 ./cmd/seed
	lipo -create -output $@ $@_arm64 $@_amd64
	@rm -f $@_arm64 $@_amd64

$(ADDUSER):
	@mkdir -p $(BINDIR)
	GOARCH=arm64 go build -o $@_arm64 ./cmd/adduser
	GOARCH=amd64 go build -o $@_amd64 ./cmd/adduser
	lipo -create -output $@ $@_arm64 $@_amd64
	@rm -f $@_arm64 $@_amd64
else
# Windows/Linux: Build single architecture
$(MAIN):
	@mkdir -p $(BINDIR)
	go build -o $@ .

$(SEED):
	@mkdir -p $(BINDIR)
	go build -o $@ ./cmd/seed

$(ADDUSER):
	@mkdir -p $(BINDIR)
	go build -o $@ ./cmd/adduser
endif

clean:
	rm -rf $(BINDIR)
