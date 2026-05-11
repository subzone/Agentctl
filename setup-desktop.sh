#!/bin/bash
# setup-desktop.sh - Initial setup for desktop development

set -e

echo "🚀 AgentCTL Desktop Setup"
echo "========================="
echo

# Check Go version
echo "📦 Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
if [[ "$(printf '%s\n' "1.26" "$GO_VERSION" | sort -V | head -n1)" != "1.26" ]]; then
    echo "❌ Go 1.26+ required, found $GO_VERSION"
    exit 1
fi
echo "✅ Go $GO_VERSION"
echo

# Check Node version
echo "📦 Checking Node version..."
if ! command -v node &> /dev/null; then
    echo "❌ Node.js not found. Please install Node.js 18+"
    exit 1
fi
NODE_VERSION=$(node --version | sed 's/v//')
echo "✅ Node $NODE_VERSION"
echo

# Install Wails CLI
echo "📦 Installing Wails CLI..."
if command -v wails &> /dev/null; then
    echo "✅ Wails already installed: $(wails version)"
else
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    echo "✅ Wails installed"
fi
echo

# Add Wails dependencies to go.mod
echo "📦 Adding Wails dependencies..."
go get github.com/wailsapp/wails/v2@latest
go mod tidy
echo "✅ Dependencies added"
echo

# Install frontend dependencies
echo "📦 Installing frontend dependencies..."
cd frontend
npm install
cd ..
echo "✅ Frontend dependencies installed"
echo

# Generate Wails bindings
echo "📦 Generating Wails bindings..."
wails generate module
echo "✅ Bindings generated"
echo

echo "🎉 Setup complete!"
echo
echo "Next steps:"
echo "  1. Test development mode:  make desktop-dev"
echo "  2. Build for your platform: make desktop-build"
echo "  3. Run CLI mode:           ./m chat"
echo "  4. Run GUI mode:           ./m --gui"
echo
echo "See DESKTOP_IMPLEMENTATION.md for full details."
