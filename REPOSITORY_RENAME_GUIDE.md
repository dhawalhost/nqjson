# Repository Rename Guide: njson → nqjson

## 🎯 Overview
This guide walks you through renaming the GitHub repository from `njson` to `nqjson`.

## ✅ Pre-Rename Checklist
- [x] All code updated to use `nqjson` package name
- [x] All files renamed from `njson_*` to `nqjson_*`
- [x] Documentation updated with new name
- [x] Brand assets created (icons, logos)
- [x] Tests passing with new name
- [ ] Commit and push all changes
- [ ] Rename repository on GitHub
- [ ] Update local repository remote URL
- [ ] Verify everything works

## 📋 Step-by-Step Instructions

### Step 1: Commit and Push Current Changes
```bash
# Check current status
git status

# Add all changes
git add .

# Commit with descriptive message
git commit -m "Complete rebranding from njson to nqjson

- Updated package name to nqjson
- Renamed all njson_* files to nqjson_*
- Updated all documentation
- Created unified brand assets with consistent typography
- Added migration guide for users"

# Push to current branch
git push origin add-multipath-modifiers-query
```

### Step 2: Rename Repository on GitHub

#### Option A: Via GitHub Web Interface (Recommended)
1. Go to your repository: https://github.com/dhawalhost/njson
2. Click **Settings** (top right, gear icon)
3. Scroll down to **Repository name** section
4. Change `njson` to `nqjson`
5. Click **Rename** button
6. GitHub will show a warning about impacts - click **I understand, rename this repository**

#### Option B: Via GitHub CLI (if installed)
```bash
# Install GitHub CLI if not already installed
# winget install GitHub.cli

# Rename repository
gh repo rename nqjson --yes
```

### Step 3: Update Local Repository Remote URL

After renaming on GitHub, update your local repository:

```bash
# Check current remote URL
git remote -v

# Update remote URL to new repository name
git remote set-url origin https://github.com/dhawalhost/nqjson.git

# Verify the change
git remote -v

# Expected output:
# origin  https://github.com/dhawalhost/nqjson.git (fetch)
# origin  https://github.com/dhawalhost/nqjson.git (push)
```

### Step 4: Update Repository Description and Details

1. Go to https://github.com/dhawalhost/nqjson
2. Click the **⚙️ gear icon** next to "About" (right sidebar)
3. Update:
   - **Description**: `nqjson - Next-gen Query JSON library for Go with powerful path queries and modifiers`
   - **Website**: (if you have documentation site)
   - **Topics**: Add tags like `json`, `golang`, `query`, `parser`, `json-path`, `go-library`
4. Click **Save changes**

### Step 5: Set Social Preview Image

1. In repository **Settings** → Scroll to **Social preview**
2. Click **Upload an image**
3. Convert and upload the social preview:
   ```bash
   # If you have ImageMagick installed:
   magick assets/social-preview.svg -density 300 -resize 1200x630 social-preview.png
   
   # Or use an online converter:
   # https://cloudconvert.com/svg-to-png
   # Upload assets/social-preview.svg, set size to 1200x630
   ```
4. Upload the PNG file

### Step 6: Update README Badge URLs (if any)

If your README.md has any badges with URLs containing the old repo name:
```bash
# Search for any remaining references to old repo URL
grep -r "github.com/dhawalhost/njson" .

# Update if needed (already done, but verify)
```

### Step 7: Create Release Announcement

```bash
# Tag the release
git tag -a v1.0.0 -m "v1.0.0 - Initial release as nqjson

Major rebranding from njson to nqjson (Next-gen Query JSON)

Features:
- Powerful JSON path queries with multi-path support
- Rich set of modifiers for data transformation
- High performance with efficient parsing
- Comprehensive test coverage
- Professional brand identity with unified icon system

See MIGRATION.md for upgrading from njson (if previously used).
See README.md for complete documentation."

# Push the tag
git push origin v1.0.0
```

Then create release on GitHub:
1. Go to https://github.com/dhawalhost/nqjson/releases
2. Click **Draft a new release**
3. Choose tag: `v1.0.0`
4. Release title: `v1.0.0 - nqjson First Release`
5. Description:
```markdown
# 🎉 nqjson v1.0.0 - Initial Release

Welcome to **nqjson** - Next-gen Query JSON library for Go!

## 🚀 What is nqjson?

A powerful, high-performance JSON library for Go with advanced path queries and data transformation capabilities.

## ✨ Key Features

- 🔍 **Advanced Path Queries**: Complex JSON traversal with multi-path support
- ⚡ **High Performance**: Optimized for speed and efficiency
- 🎯 **Rich Modifiers**: Transform data on-the-fly with built-in modifiers
- 📦 **Zero Dependencies**: Pure Go implementation
- ✅ **Well Tested**: Comprehensive test coverage
- 📚 **Complete Documentation**: Extensive guides and examples

## 📦 Installation

```bash
go get github.com/dhawalhost/nqjson
```

## 📖 Documentation

- [README.md](README.md) - Getting started guide
- [API.md](API.md) - Complete API reference
- [EXAMPLES.md](EXAMPLES.md) - Usage examples
- [SYNTAX.md](SYNTAX.md) - Query syntax guide
- [BENCHMARKS.md](BENCHMARKS.md) - Performance benchmarks

## 🔄 Migration from njson

If you were using the previous `njson` name, see [MIGRATION.md](MIGRATION.md) for upgrade instructions.

## 🙏 Acknowledgments

Built with ❤️ for the Go community.
```

### Step 8: Verify Everything Works

```bash
# Test that module can be fetched with new name
go get github.com/dhawalhost/nqjson@v1.0.0

# In a test project:
mkdir /tmp/test-nqjson
cd /tmp/test-nqjson
go mod init test
go get github.com/dhawalhost/nqjson
# Should work without errors!
```

## 🎯 Post-Rename Actions

### Immediate
- [ ] Update any external documentation linking to old repo
- [ ] Update any CI/CD configurations if they reference repo URL
- [ ] Announce on social media / Go forums (optional)

### For Users
- [ ] Notify existing users (if any) about the rename
- [ ] Keep MIGRATION.md updated
- [ ] Monitor issues for migration questions

## ⚠️ Important Notes

### GitHub Redirects
✅ GitHub automatically creates redirects from old URL to new URL
- `github.com/dhawalhost/njson` → `github.com/dhawalhost/nqjson`
- Old clone URLs will still work temporarily
- **But**: Update your remotes to use the new URL for best practice

### Go Module Path
✅ Module path already updated in go.mod
- Old: `github.com/dhawalhost/njson`
- New: `github.com/dhawalhost/nqjson`
- Users will need to update imports (see MIGRATION.md)

### Existing Clones
⚠️ Anyone with the old repository cloned needs to:
1. Update their remote URL (Step 3 above)
2. Or re-clone from new URL

## 🆘 Troubleshooting

### Problem: "Repository not found" after rename
**Solution**: Update remote URL (Step 3)

### Problem: Go modules still reference old path
**Solution**: 
```bash
go clean -modcache
go get github.com/dhawalhost/nqjson@latest
```

### Problem: CI/CD pipeline failing
**Solution**: Update repository URLs in CI configuration files

## 📞 Need Help?

If you encounter issues during the rename:
1. Check GitHub's redirect is working
2. Verify your remote URL is updated
3. Clear Go module cache
4. Re-clone if needed

## ✅ Completion Checklist

After completing all steps:
- [ ] Repository renamed on GitHub
- [ ] Local remote URL updated
- [ ] Repository description updated
- [ ] Social preview image set
- [ ] v1.0.0 release created
- [ ] Everything tested and working
- [ ] This guide can be archived

---

**Ready to rename?** Start with Step 1! 🚀
