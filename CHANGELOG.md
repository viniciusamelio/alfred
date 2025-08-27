# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Automatic upstream configuration for push and pull commands
- Interactive commit interface with diff visualization
- Diagnostic command to troubleshoot git repository issues
- Comprehensive installation script for macOS and Linux
- CI/CD pipeline with GitHub Actions
- Multi-platform builds (macOS, Linux, Windows)
- Makefile with development commands
- Shell completion support (bash, zsh, fish)
- Security scanning and code quality checks

### Enhanced
- Improved error messages with detailed git output
- Better upstream detection and configuration
- Enhanced pull command with automatic upstream setup
- Push command with intelligent upstream handling

### Fixed
- Remote branch existence check before setting upstream
- Better error handling for git operations
- Improved cross-platform compatibility

## [1.0.0] - 2025-01-XX

### Added
- Initial release of Alfred CLI
- Multi-repository management for Flutter/Dart projects
- Context-based workflow management
- Git worktree and branch mode support
- Dependency management between repositories
- Production preparation with git dependency reversion
- Interactive repository scanning and configuration
- Comprehensive CLI interface with Kong framework

### Features
- **Context Management**: Create, switch, and manage development contexts
- **Worktree Support**: Isolated development environments per context
- **Branch Mode**: Alternative workflow using git branches
- **Dependency Sync**: Automatic pubspec.yaml dependency management
- **Interactive Setup**: Guided repository discovery and configuration
- **Production Ready**: Automated preparation for deployment

### Supported Platforms
- macOS (Intel and Apple Silicon)
- Linux (x64 and ARM64)
- Windows (x64)