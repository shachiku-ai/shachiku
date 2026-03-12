.PHONY: build build-linux-amd64 build-darwin-arm64 build-windows-amd64 build-ui clean

# Default build task
all: build-ui build-linux-amd64 build-darwin-arm64 build-windows-amd64

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
build-linux-amd64: build-ui
	@echo "========================================"
	@echo "3. Compiling shachiku to dist directory (Linux amd64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku && go mod tidy && GOOS=linux GOARCH=amd64 go build -o ../dist/shachiku-linux-amd64 main.go
	@echo "========================================"
	@echo "✨ Linux amd64 build complete!"
	@echo "🎯 Executable path: dist/shachiku-linux-amd64"
	@echo "💡 Tip: UI resources have been packed into the binary via go:embed"
	@echo "========================================"

# Main compilation method: Cross-platform build (Darwin ARM64 / Apple Silicon)
build-darwin-arm64: build-ui
	@echo "========================================"
	@echo "3. Compiling shachiku to dist directory (Darwin arm64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku && go mod tidy && GOOS=darwin GOARCH=arm64 go build -o ../dist/shachiku-darwin-arm64 main.go
	@echo "========================================"
	@echo "✨ Darwin arm64 build complete!"
	@echo "🎯 Executable path: dist/shachiku-darwin-arm64"
	@echo "💡 Tip: UI resources have been packed into the binary via go:embed"
	@echo "========================================"

# Main compilation method: Cross-platform build (Windows AMD64)
build-windows-amd64: build-ui
	@echo "========================================"
	@echo "3. Compiling shachiku to dist directory (Windows amd64)..."
	@echo "========================================"
	mkdir -p dist
	cd shachiku && go mod tidy && GOOS=windows GOARCH=amd64 go build -o ../dist/shachiku-windows-amd64.exe main.go
	@echo "========================================"
	@echo "✨ Windows amd64 build complete!"
	@echo "🎯 Executable path: dist/shachiku-windows-amd64.exe"
	@echo "💡 Tip: UI resources have been packed into the binary via go:embed"
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
