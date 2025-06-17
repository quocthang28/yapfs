# YAPFS Makefile
# Yet Another P2P File Sharing

# Build variables
BINARY_NAME=yapfs
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .