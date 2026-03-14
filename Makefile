.PHONY: build build-linux-amd64 build-darwin-arm64 build-windows-amd64 build-darwin-desktop build-windows-desktop build-ui clean

TARGET_VERSION ?= dev
VERSION ?= $(patsubst v%,%,$(TARGET_VERSION))
LDFLAGS := -ldflags="-X main.Version=$(VERSION)"

# Default build task
all: build-ui build-linux-amd64 build-darwin-arm64 build-windows-amd64 build-darwin-desktop build-windows-desktop

build-ui:
	@echo "========================================"
	@echo "1. Building shachiku-ui (Next.js Static Export)..."
	@echo "========================================"
	cd shachiku-ui && pnpm install && pnpm run build
	
	@echo "========================================"
	@echo "2. Moving shachiku-ui build artifacts to shachiku/ui/dist..."
	@echo "========================================"
	# Ensure the directory exists and clean old static files (keep .gitkeep to avoid Go build errors)
	mkdir -p shachiku/ui/dist
	find shachiku/ui/dist -type f -not -name '.gitkeep' -delete
	find shachiku/ui/dist -mindepth 1 -type d -empty -delete
	# Copy the latest UI build artifacts (Next.js exports to 'out' directory by default with output: 'export')
	cp -r shachiku-ui/out/* shachiku/ui/dist/

# Main compilation method: Cross-platform build (Linux AMD64)
build-linux-amd64:
	@echo "========================================"
	@echo "3. Compiling shachiku to dist directory (Linux amd64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku && go mod tidy && GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ../dist/shachiku-linux-amd64 main.go
	@echo "========================================"
	@echo "✨ Linux amd64 build complete!"
	@echo "🎯 Executable path: dist/shachiku-linux-amd64"
	@echo "💡 Tip: UI resources have been packed into the binary via go:embed"
	@echo "========================================"

# Main compilation method: Cross-platform build (Darwin ARM64 / Apple Silicon)
build-darwin-arm64:
	@echo "========================================"
	@echo "3. Compiling shachiku to dist directory (Darwin arm64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku && go mod tidy && GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o ../dist/shachiku-darwin-arm64 main.go
	@echo "========================================"
	@echo "✨ Darwin arm64 build complete!"
	@echo "🎯 Executable path: dist/shachiku-darwin-arm64"
	@echo "💡 Tip: UI resources have been packed into the binary via go:embed"
	@echo "========================================"

# Main compilation method: Cross-platform build (Windows AMD64)
build-windows-amd64:
	@echo "========================================"
	@echo "3. Compiling shachiku to dist directory (Windows amd64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku && go mod tidy && GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../dist/shachiku-windows-amd64.exe main.go
	@echo "========================================"
	@echo "✨ Windows amd64 build complete!"
	@echo "🎯 Executable path: dist/shachiku-windows-amd64.exe"
	@echo "💡 Tip: UI resources have been packed into the binary via go:embed"
	@echo "========================================"

# Desktop client build using Wails v3
build-darwin-desktop:
	@echo "========================================"
	@echo "3. Compiling shachiku-desktop (Darwin arm64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku-desktop && wails3 task darwin:package ARCH=arm64 LDFLAGS=$(LDFLAGS)
	rm -rf dist/Shachiku.app
	cp -a shachiku-desktop/bin/shachiku-desktop.app dist/Shachiku.app
	@echo "========================================"
	@echo "4. Creating DMG image..."
	@echo "========================================"
	rm -f dist/shachiku-desktop-macos-arm64.dmg
	set -o pipefail && create-dmg \
		--volname "Shachiku" \
		--window-pos 200 120 \
		--window-size 600 400 \
		--icon-size 100 \
		--icon "Shachiku.app" 150 190 \
		--no-internet-enable \
		--hide-extension "Shachiku.app" \
		--app-drop-link 450 190 \
		"dist/shachiku-desktop-macos-arm64.dmg" \
		"dist/Shachiku.app" | grep -v "Not setting 'internet-enable'"
	@echo "========================================"
	@echo "✨ Desktop builds complete!"
	@echo "🎯 Executable path: dist/"
	@echo "========================================"

# Desktop client build using Wails v3 (Windows)
build-windows-desktop:
	@echo "========================================"
	@echo "3. Compiling shachiku-desktop (Windows amd64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku-desktop && wails3 task windows:build ARCH=amd64 LDFLAGS=$(LDFLAGS)
	cp -a shachiku-desktop/bin/shachiku-desktop.exe dist/shachiku-desktop-windows-amd64.exe
	@echo "========================================"
	@echo "✨ Windows Desktop build complete!"
	@echo "🎯 Executable path: dist/shachiku-desktop-windows-amd64.exe"
	@echo "========================================"

# Clean build artifacts
clean:
	@echo "Cleaning build directories..."
	rm -rf dist
	find shachiku/ui/dist -type f -not -name '.gitkeep' -delete
	find shachiku/ui/dist -type d -empty -delete
	rm -rf shachiku-ui/out
	rm -rf shachiku-ui/.next
	@echo "Clean complete!"