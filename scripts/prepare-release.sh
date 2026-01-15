#!/bin/bash
#
# Prepare RavenForge repository for GitHub release
#

set -e

echo "Preparing RavenForge for GitHub release..."

# Clean build artifacts
echo "Cleaning build artifacts..."
rm -rf core/bin
rm -f core/ravenforged core/ravenforge
rm -f core/ravenforged.exe core/ravenforge.exe
rm -f core/coverage.out core/coverage.html

# Clean test data
rm -rf test-data
rm -rf test-output

# Clean generated configs
rm -f core/config/ravenforge.yaml

# Clean Python cache
find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
find . -name "*.pyc" -delete 2>/dev/null || true

# Create placeholder for logo
mkdir -p docs/assets
if [[ ! -f docs/assets/logo.png ]]; then
    echo "Note: Please add your logo to docs/assets/logo.png"
fi

# Verify required files exist
required_files=(
    "README.md"
    "LICENSE"
    "CONTRIBUTING.md"
    "SECURITY.md"
    ".gitignore"
    "Makefile"
    "PKGBUILD"
    "go.mod"
    "scripts/install-arch.sh"
    "scripts/ravenforged.service"
    "scripts/uninstall.sh"
    "core/config/ravenforge.linux.yaml"
    "core/config/ravenforge.example.yaml"
    "docs/ARCHITECTURE.md"
    "docs/TOOL_DEVELOPMENT.md"
    "docs/CONFIGURATION.md"
    "docs/ARCH_LINUX_INSTALL.md"
)

missing=0
for file in "${required_files[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo "Missing: $file"
        missing=1
    fi
done

if [[ $missing -eq 1 ]]; then
    echo "Some required files are missing!"
    exit 1
fi

# Make scripts executable
chmod +x scripts/*.sh

echo ""
echo "Repository is ready for GitHub!"
echo ""
echo "Next steps:"
echo "  1. Initialize git repository:"
echo "     git init"
echo "     git add ."
echo "     git commit -m 'Initial commit'"
echo ""
echo "  2. Create GitHub repository and push:"
echo "     git remote add origin https://github.com/YOUR_USERNAME/ravenforge.git"
echo "     git branch -M main"
echo "     git push -u origin main"
echo ""
echo "  3. Create a release tag:"
echo "     git tag -a v1.0.0 -m 'Release v1.0.0'"
echo "     git push origin v1.0.0"
echo ""
echo "  4. Update PKGBUILD with your username:"
echo "     sed -i 's/yourusername/YOUR_USERNAME/g' PKGBUILD"
echo ""
