#!/bin/bash
# Quick fix script for critical linting issues before setting up pre-commit

echo "🔧 Fixing critical linting issues..."

# Fix formatting
echo "📝 Running gofmt..."
gofmt -w .

# Fix imports
echo "📦 Running goimports..."
goimports -w .

echo "✅ Basic formatting fixes complete!"
echo "💡 Some unused code warnings remain - these can be addressed later"
echo "🚀 Ready to set up pre-commit hook!"
