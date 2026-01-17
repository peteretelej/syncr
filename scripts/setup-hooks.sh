#!/bin/bash
# Set up git hooks for syncr

set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
HOOKS_DIR="$REPO_ROOT/.git/hooks"

echo "Setting up git hooks..."

# Create pre-push hook
cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash
# Pre-push hook - runs lint, build, tests before push

exec "$(git rev-parse --show-toplevel)/scripts/prepush.sh"
EOF

chmod +x "$HOOKS_DIR/pre-push"

echo "✓ Pre-push hook installed"
echo ""
echo "The following checks will run before each push:"
echo "  - gofmt (formatting)"
echo "  - go vet"
echo "  - staticcheck (if installed)"
echo "  - go build"
echo "  - go test"
