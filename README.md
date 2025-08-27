# Alfred CLI

[![CI/CD Pipeline](https://github.com/viniciusamelio/alfred/actions/workflows/ci.yml/badge.svg)](https://github.com/viniciusamelio/alfred/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/viniciusamelio/alfred)](https://goreportcard.com/report/github.com/viniciusamelio/alfred)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/viniciusamelio/alfred.svg)](https://github.com/viniciusamelio/alfred/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/viniciusamelio/alfred)](https://golang.org/)

A powerful CLI tool for managing multi-repository Flutter/Dart projects with context-based workflows, enabling coordinated development across multiple repositories through intelligent context switching and dependency management.

## ✨ Features

- **🎯 Context Management**: Create and switch between different development contexts
- **🌿 Git Worktrees & Branches**: Support for both git worktrees and branch-based workflows  
- **📦 Dependency Management**: Automatic dependency synchronization between repositories
- **💻 Interactive Commit Interface**: Visual interface for committing changes across multiple repositories
- **🚀 Production Ready**: Automated preparation for deployment with git dependency reversion
- **🔄 Automatic Upstream**: Intelligent upstream configuration for push/pull operations
- **🔍 Diagnostics**: Built-in troubleshooting tools for repository status

## 🚀 Installation

### Secure Installation Script (Recommended)

Run our secure installation script that works on **macOS** and **Linux** with **bash**, **zsh**, and **fish**:

```bash
curl -fsSL https://raw.githubusercontent.com/viniciusamelio/alfred/main/scripts/install.sh | bash
```

Or with wget:

```bash
wget -qO- https://raw.githubusercontent.com/viniciusamelio/alfred/main/scripts/install.sh | bash
```

**Security Features:**

- ✅ Integrity verification with checksums
- ✅ Never runs as root for security
- ✅ Automatic OS and architecture detection
- ✅ macOS Gatekeeper compatible
- ✅ Shell completion setup (bash, zsh, fish)

### Installation Options

```bash
# Install specific version
curl -fsSL https://raw.githubusercontent.com/viniciusamelio/alfred/main/scripts/install.sh | bash -s -- -v v1.2.3

# Custom installation directory
ALFRED_INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/viniciusamelio/alfred/main/scripts/install.sh | bash

# Uninstall
curl -fsSL https://raw.githubusercontent.com/viniciusamelio/alfred/main/scripts/install.sh | bash -s -- --uninstall
```

### Alternative Installation Methods

**Go Install:**

```bash
go install github.com/viniciusamelio/alfred@latest
```

**Manual Download:**
Download pre-built binaries from [releases page](https://github.com/viniciusamelio/alfred/releases/latest)

**Build from Source:**

```bash
git clone https://github.com/viniciusamelio/alfred
cd alfred
make build
```

## 🚀 Getting Started

### 1. Initialize Alfred in your project directory

```bash
# Scan and auto-configure existing Dart/Flutter packages
alfred scan

# Or initialize with manual configuration
alfred init
```

### 2. Create and switch to a development context

```bash
# Create a new context
alfred create

# Switch to a context
alfred switch my-feature
```

### 3. Work with your repositories

```bash
# Interactive commit across all repositories
alfred commit

# Push with automatic upstream configuration
alfred push

# Pull with automatic upstream setup
alfred pull

# Diagnose repository issues
alfred diagnose
```

## 📖 Usage

### Context Management

```bash
alfred list                    # List available contexts
alfred create                  # Create a new context
alfred switch <context-name>   # Switch to a context
alfred switch main             # Switch to main/master branches
alfred status                  # Show current status
```

### Repository Operations

```bash
alfred commit                  # Interactive commit interface
alfred push                    # Push with automatic upstream
alfred pull                    # Pull with automatic upstream
alfred diagnose                # Troubleshoot repository issues
```

### Advanced Features

```bash
alfred prepare                 # Prepare for production deployment
alfred main-branch <branch>    # Set main branch name
```

## 🛠️ Development

### Prerequisites

- Go 1.21+
- Make
- Git

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/viniciusamelio/alfred
cd alfred

# Install dependencies
make deps

# Run tests
make test

# Build locally
make build

# Development with hot reload
make dev
```

### Available Make Commands

```bash
make build          # Build the binary
make build-all      # Build for all platforms
make test           # Run tests
make coverage       # Tests with coverage
make lint           # Lint code
make fmt            # Format code
make security       # Security checks
make install        # Install to system
make uninstall      # Uninstall from system
make release        # Create release archives
make clean          # Clean artifacts
make help           # Show all commands
```

## 🤝 Contributing

Contributions are welcome! Please follow these steps:

1. Fork the project
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Contributing Guidelines

- Write tests for new features
- Keep code formatted (`make fmt`)
- Run linting (`make lint`)
- Update documentation when necessary
- Follow the project's commit conventions
- Ensure CI/CD pipeline passes

### Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Follow the project's coding standards

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <strong>Made with ❤️ for the Flutter/Dart community</strong>
</p>
