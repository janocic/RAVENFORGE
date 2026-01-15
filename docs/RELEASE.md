# RavenForge Release Checklist

## Pre-Release Checklist

### Code Quality
- [ ] All tests pass (`make test`)
- [ ] No linting errors (`make lint`)
- [ ] Code formatted (`make fmt`)
- [ ] No hardcoded credentials or paths
- [ ] All TODO/FIXME resolved

### Documentation
- [ ] README.md is up to date
- [ ] CHANGELOG.md updated with changes
- [ ] API documentation updated
- [ ] Installation guides tested
- [ ] All links working

### Configuration
- [ ] Example configs provided
- [ ] Linux config tested
- [ ] Default values are sensible
- [ ] Sensitive defaults are secure

### Build
- [ ] Builds on Linux AMD64
- [ ] Builds on Linux ARM64
- [ ] Builds on macOS
- [ ] Docker images build successfully
- [ ] No build warnings

### Security
- [ ] No sensitive data in repository
- [ ] Dependencies up to date
- [ ] Security advisory checked
- [ ] SECURITY.md is accurate

## Release Process

### 1. Update Version

```bash
# Update version in relevant files
VERSION="1.0.0"
sed -i "s/pkgver=.*/pkgver=$VERSION/" PKGBUILD
```

### 2. Create CHANGELOG Entry

```markdown
## [1.0.0] - 2026-01-15

### Added
- Initial release
- Tool registry with discovery
- Docker sandbox execution
- Audit logging with hash chain
- Policy engine
- Pipeline executor
- REST API
- CLI

### Tools
- ingest-jsonlines
- detect-simple-rules
- enrich-geoip
- correlate-events
- report-generate
- triage-prioritize
```

### 3. Commit and Tag

```bash
git add .
git commit -m "Release v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main --tags
```

### 4. GitHub Release

1. Go to GitHub Releases
2. Create new release from tag
3. Write release notes
4. Attach binaries (optional, CI builds them)

### 5. Post-Release

- [ ] Verify GitHub Actions succeeded
- [ ] Test installation from GitHub
- [ ] Announce release
- [ ] Update AUR package (if applicable)

## Version Numbering

We follow Semantic Versioning (SemVer):

- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

## Hotfix Process

For critical bugs:

```bash
git checkout -b hotfix/critical-bug
# Fix the bug
git commit -m "Fix: critical security issue"
git checkout main
git merge hotfix/critical-bug
git tag -a v1.0.1 -m "Hotfix v1.0.1"
git push origin main --tags
```
