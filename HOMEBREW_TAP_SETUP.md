# Homebrew Tap Setup Instructions

This document provides step-by-step instructions for setting up the Homebrew tap for graphqls-to-asciidoc.

## 🚀 Quick Start

The Homebrew tap has been pre-configured and is ready to use. Users can install the tool with:

```bash
brew tap bovinemagnet/tap
brew install graphqls-to-asciidoc
```

## 📁 Files Created

### Main Repository (graphqls-to-asciidoc)

1. **`.goreleaser.yml`** - GoReleaser configuration that handles:
   - Cross-platform builds (macOS, Linux, Windows)
   - Archive creation with proper naming
   - Automatic Homebrew formula updates
   - GitHub release management

2. **`.github/workflows/release.yml`** - Updated to include Homebrew tap token

3. **`homebrew-formula/graphqls-to-asciidoc.rb`** - Template formula for reference

### Homebrew Tap Repository (`/Users/paul/gitHub/homebrew-tap/`)

1. **`Formula/graphqls-to-asciidoc.rb`** - Main Homebrew formula
2. **`README.md`** - Tap documentation and usage instructions  
3. **`SETUP.md`** - Detailed setup and maintenance guide
4. **`LICENSE`** - MIT license
5. **`.gitignore`** - Standard ignore patterns
6. **`.github/workflows/update-formula.yml`** - CI for formula validation

## 🔧 Setup Steps

### 1. Create Homebrew Tap Repository

```bash
# Navigate to the tap directory
cd /Users/paul/gitHub/homebrew-tap

# Initialize git repository
git init
git add .
git commit -m "Initial homebrew tap setup"
git branch -M main

# Create GitHub repository and push
# Repository name: homebrew-tap
# Organization: bovinemagnet
git remote add origin git@github.com:bovinemagnet/homebrew-tap.git
git push -u origin main
```

### 2. Configure Repository Secrets

In the **main repository** (graphqls-to-asciidoc), add a GitHub secret:

- **Name**: `HOMEBREW_TAP_GITHUB_TOKEN`
- **Value**: Personal access token with `repo` permissions for the tap repository

### 3. Test the Release Process

```bash
# In the main repository
git tag v0.1.0
git push origin v0.1.0
```

This will trigger:
1. GitHub Actions workflow
2. GoReleaser build and release
3. Automatic Homebrew formula update

## 📋 Formula Features

The Homebrew formula includes:

- **Multi-platform support**: macOS (Intel/ARM), Linux (Intel/ARM)
- **Automatic SHA256 verification**: Ensures package integrity
- **Comprehensive testing**: Version check and functionality tests
- **Proper dependencies**: No external dependencies required

## 🧪 Testing

### Local Formula Testing

```bash
# Test formula syntax
brew ruby -c Formula/graphqls-to-asciidoc.rb

# Install from local formula  
brew install --build-from-source ./Formula/graphqls-to-asciidoc.rb

# Run formula tests
brew test graphqls-to-asciidoc

# Clean up
brew uninstall graphqls-to-asciidoc
```

### User Installation Testing

```bash
# Test tap installation
brew tap bovinemagnet/tap
brew install graphqls-to-asciidoc

# Verify installation
graphqls-to-asciidoc -version
```

## 🔄 Release Workflow

1. **Development**: Make changes in main repository
2. **Testing**: Run tests locally and via CI
3. **Tagging**: Create version tag (`git tag v1.0.0`)
4. **Release**: Push tag to trigger automated release
5. **Formula Update**: GoReleaser automatically updates Homebrew formula
6. **User Access**: Users can immediately install the new version

## 📚 Documentation Updates

The main README.md has been updated to include Homebrew installation as the primary method:

```bash
### Homebrew (macOS and Linux)
brew tap bovinemagnet/tap
brew install graphqls-to-asciidoc
```

## 🛡️ Security

- SHA256 checksums validate package integrity
- All downloads come from official GitHub releases
- Formula validates before installation
- No external dependencies or network calls during runtime

## 🆘 Troubleshooting

### Common Issues

1. **Formula not found**: Ensure tap is properly added
2. **SHA256 mismatch**: Wait for GoReleaser to update checksums
3. **Version mismatch**: Check if GitHub release exists

### Debug Commands

```bash
# Check tap status
brew tap-info bovinemagnet/tap

# Validate formula
brew audit bovinemagnet/tap/graphqls-to-asciidoc

# Verbose installation
brew install --verbose bovinemagnet/tap/graphqls-to-asciidoc
```

## 🔗 Links

- **Tap Repository**: `git@github.com:bovinemagnet/homebrew-tap.git`
- **Main Repository**: `git@github.com:bovinemagnet/graphqls-to-asciidoc.git`
- **Homebrew Docs**: https://docs.brew.sh/Formula-Cookbook
- **GoReleaser Docs**: https://goreleaser.com/customization/homebrew/

The Homebrew tap is now ready for use and will automatically stay updated with new releases!