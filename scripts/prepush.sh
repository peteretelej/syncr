#!/bin/bash
# Pre-push hook script for syncr
# Runs lint, build, and tests before allowing push

set -e

echo "=== Pre-push checks ==="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Change to repo root
cd "$(git rev-parse --show-toplevel)"

# 1. Format check
echo ""
echo "Checking formatting..."
if ! gofmt -l . | grep -q .; then
    echo -e "${GREEN}✓ Formatting OK${NC}"
else
    echo -e "${RED}✗ Formatting issues found:${NC}"
    gofmt -l .
    echo "Run 'gofmt -w .' to fix"
    exit 1
fi

# 2. Vet
echo ""
echo "Running go vet..."
if go vet ./...; then
    echo -e "${GREEN}✓ Vet OK${NC}"
else
    echo -e "${RED}✗ Vet failed${NC}"
    exit 1
fi

# 3. Staticcheck (if installed)
echo ""
if command -v staticcheck &> /dev/null; then
    echo "Running staticcheck..."
    if staticcheck ./...; then
        echo -e "${GREEN}✓ Staticcheck OK${NC}"
    else
        echo -e "${RED}✗ Staticcheck failed${NC}"
        exit 1
    fi
else
    echo "Staticcheck not installed, skipping (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"
fi

# 4. Build
echo ""
echo "Building..."
if go build -o /dev/null .; then
    echo -e "${GREEN}✓ Build OK${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# 5. Tests
echo ""
echo "Running tests..."
if go test ./...; then
    echo -e "${GREEN}✓ Tests OK${NC}"
else
    echo -e "${RED}✗ Tests failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}=== All checks passed ===${NC}"
