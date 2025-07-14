# Release Process

This document describes how to create a new release of scripto.

## Automated Release Process

The project uses GitHub Actions to automatically build and release binaries when you push a version tag.

### Creating a Release

1. **Update the changelog:**
   ```bash
   # Edit CHANGELOG.md to add the new version section
   vim CHANGELOG.md
   ```

2. **Commit your changes:**
   ```bash
   git add CHANGELOG.md
   git commit -m "Prepare release v1.0.0"
   git push origin main
   ```

3. **Create and push a version tag:**
   ```bash
   # Create a new tag (use semantic versioning)
   git tag v1.0.0
   
   # Push the tag to trigger the release workflow
   git push origin v1.0.0
   ```

4. **Monitor the build:**
   - Go to the [Actions tab](../../actions) in GitHub
   - Watch the "Release" workflow complete
   - The release will be automatically created with binaries attached

### What the Automation Does

The GitHub Action workflow will:

1. ✅ **Build** - Compile ARM64 macOS binary with version info
2. ✅ **Test** - Run all tests to ensure quality
3. ✅ **Package** - Create tar.gz archive with binary, README, and shell scripts
4. ✅ **Checksums** - Generate SHA256 checksums for verification
5. ✅ **Release** - Create GitHub release with:
   - Release notes from CHANGELOG.md
   - Installation instructions
   - Binary attachments
   - Checksum files

### Available Binaries

Currently supported platforms:
- **macOS ARM64** (`darwin/arm64`) - Apple Silicon Macs (M1, M2, M3)

### Version Tag Format

Use semantic versioning tags:
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)
- `v2.0.0-beta.1` - Pre-release

### Manual Release (if needed)

If you need to create a release manually:

```bash
# Build the binary
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.version=v1.0.0" -o scripto-v1.0.0-darwin-arm64 .

# Create archive
mkdir -p release
cp scripto-v1.0.0-darwin-arm64 release/scripto
cp README.md release/
cp commands/scripts/scripto.zsh release/
tar -czf scripto-v1.0.0-darwin-arm64.tar.gz -C release .

# Generate checksum
shasum -a 256 scripto-v1.0.0-darwin-arm64.tar.gz > scripto-v1.0.0-darwin-arm64.tar.gz.sha256
```

Then upload to GitHub releases manually.

## Troubleshooting

### Build Fails
- Check that all tests pass locally: `go test ./...`
- Ensure `go.mod` is up to date: `go mod tidy`
- Verify the tag follows semantic versioning

### Release Not Created
- Ensure you pushed the tag: `git push origin v1.0.0`
- Check the Actions tab for error logs
- Verify you have write permissions to the repository

### Missing Changelog
- The workflow will create a generic release note if CHANGELOG.md doesn't have an entry
- Add a section in CHANGELOG.md matching the tag version

## Examples

### Patch Release (Bug Fix)
```bash
# Update CHANGELOG.md with bug fixes
git add CHANGELOG.md
git commit -m "Fix script execution bug"
git push origin main

# Create patch version tag
git tag v1.0.1
git push origin v1.0.1
```

### Feature Release
```bash
# Update CHANGELOG.md with new features
git add CHANGELOG.md
git commit -m "Add new script management features"
git push origin main

# Create minor version tag
git tag v1.1.0
git push origin v1.1.0
```

### Major Release
```bash
# Update CHANGELOG.md with breaking changes
git add CHANGELOG.md
git commit -m "Major refactor with breaking changes"
git push origin main

# Create major version tag
git tag v2.0.0
git push origin v2.0.0
```