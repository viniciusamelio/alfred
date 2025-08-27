#!/bin/bash

# Alfred CLI Installation Script
# This script installs Alfred CLI on macOS and Linux systems
# Supports bash, zsh, and fish shells

set -e

# Configuration
REPO="viniciusamelio/alfred"
BINARY_NAME="alfred"
INSTALL_DIR="/usr/local/bin"
LATEST_VERSION="latest"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -eq 0 ]]; then
        log_error "This script should not be run as root for security reasons."
        log_info "Please run as a regular user. The script will ask for sudo when needed."
        exit 1
    fi
}

# Detect OS and architecture
detect_platform() {
    local os arch
    
    case "$(uname -s)" in
        Darwin*)
            os="darwin"
            ;;
        Linux*)
            os="linux"
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    
    echo "${os}-${arch}"
}

# Get latest release version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        log_error "Neither curl nor wget is available. Please install one of them."
        exit 1
    fi
}

# Download and install binary
install_binary() {
    local platform="$1"
    local version="$2"
    local temp_dir
    
    temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT
    
    local download_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${version}-${platform}.tar.gz"
    local archive_file="${temp_dir}/${BINARY_NAME}.tar.gz"
    
    log_info "Downloading Alfred CLI ${version} for ${platform}..."
    
    if command -v curl >/dev/null 2>&1; then
        if ! curl -L -o "$archive_file" "$download_url"; then
            log_error "Failed to download Alfred CLI"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -O "$archive_file" "$download_url"; then
            log_error "Failed to download Alfred CLI"
            exit 1
        fi
    fi
    
    log_info "Extracting archive..."
    tar -xzf "$archive_file" -C "$temp_dir"
    
    # Find the binary (it might have platform suffix)
    local binary_path
    
    # Try different methods to find executable files for cross-platform compatibility
    # Method 1: Try -executable (GNU find)
    binary_path=$(find "$temp_dir" -name "${BINARY_NAME}*" -type f -executable 2>/dev/null | head -n1)
    
    # Method 2: Try -perm +111 (older BSD find)
    if [[ -z "$binary_path" ]]; then
        binary_path=$(find "$temp_dir" -name "${BINARY_NAME}*" -type f -perm +111 2>/dev/null | head -n1)
    fi
    
    # Method 3: Try -perm /111 (newer BSD find)
    if [[ -z "$binary_path" ]]; then
        binary_path=$(find "$temp_dir" -name "${BINARY_NAME}*" -type f -perm /111 2>/dev/null | head -n1)
    fi
    
    # Method 4: Fallback - find any file and check if it's executable
    if [[ -z "$binary_path" ]]; then
        for file in $(find "$temp_dir" -name "${BINARY_NAME}*" -type f); do
            if [[ -x "$file" ]]; then
                binary_path="$file"
                break
            fi
        done
    fi
    
    # Method 5: Last resort - just find the file (we'll make it executable later)
    if [[ -z "$binary_path" ]]; then
        binary_path=$(find "$temp_dir" -name "${BINARY_NAME}*" -type f | head -n1)
    fi
    
    if [[ ! -f "$binary_path" ]]; then
        log_error "Binary not found in archive"
        exit 1
    fi
    
    log_info "Installing Alfred CLI to ${INSTALL_DIR}..."
    
    # Create install directory if it doesn't exist
    if [[ ! -d "$INSTALL_DIR" ]]; then
        sudo mkdir -p "$INSTALL_DIR"
    fi
    
    # Install binary
    sudo cp "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
    sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    
    log_success "Alfred CLI installed successfully!"
}

# Check if install directory is in PATH
check_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        log_warning "$INSTALL_DIR is not in your PATH"
        log_info "Add the following line to your shell configuration file:"
        echo ""
        echo "export PATH=\"$INSTALL_DIR:\$PATH\""
        echo ""
        
        # Detect shell and provide specific instructions
        local shell_name
        shell_name=$(basename "$SHELL")
        
        case "$shell_name" in
            bash)
                log_info "For bash, add it to ~/.bashrc or ~/.bash_profile"
                ;;
            zsh)
                log_info "For zsh, add it to ~/.zshrc"
                ;;
            fish)
                log_info "For fish, run: fish_add_path $INSTALL_DIR"
                ;;
            *)
                log_info "Add it to your shell's configuration file"
                ;;
        esac
        
        echo ""
        log_info "Then restart your terminal or run: source ~/.${shell_name}rc"
        return 1
    fi
    return 0
}

# Verify installation
verify_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local version
        version=$("$BINARY_NAME" version 2>/dev/null || echo "unknown")
        log_success "Alfred CLI is working! Version: $version"
        log_info "Run 'alfred --help' to get started"
        return 0
    else
        log_error "Alfred CLI installation verification failed"
        return 1
    fi
}

# Setup shell completion (optional)
setup_completion() {
    local shell_name
    shell_name=$(basename "$SHELL")
    
    case "$shell_name" in
        bash)
            if [[ -d "/usr/local/etc/bash_completion.d" ]]; then
                log_info "Setting up bash completion..."
                alfred completion bash | sudo tee /usr/local/etc/bash_completion.d/alfred >/dev/null 2>&1 || true
            fi
            ;;
        zsh)
            if [[ -d "/usr/local/share/zsh/site-functions" ]]; then
                log_info "Setting up zsh completion..."
                alfred completion zsh | sudo tee /usr/local/share/zsh/site-functions/_alfred >/dev/null 2>&1 || true
            fi
            ;;
        fish)
            if [[ -d "$HOME/.config/fish/completions" ]]; then
                log_info "Setting up fish completion..."
                mkdir -p "$HOME/.config/fish/completions"
                alfred completion fish > "$HOME/.config/fish/completions/alfred.fish" 2>/dev/null || true
            fi
            ;;
    esac
}

# Uninstall function
uninstall() {
    log_info "Uninstalling Alfred CLI..."
    
    if [[ -f "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        sudo rm -f "${INSTALL_DIR}/${BINARY_NAME}"
        log_success "Alfred CLI uninstalled successfully"
    else
        log_warning "Alfred CLI is not installed"
    fi
    
    # Remove completions
    sudo rm -f "/usr/local/etc/bash_completion.d/alfred" 2>/dev/null || true
    sudo rm -f "/usr/local/share/zsh/site-functions/_alfred" 2>/dev/null || true
    rm -f "$HOME/.config/fish/completions/alfred.fish" 2>/dev/null || true
    
    exit 0
}

# Show help
show_help() {
    cat << EOF
Alfred CLI Installation Script

Usage: $0 [OPTIONS]

Options:
    -h, --help      Show this help message
    -u, --uninstall Uninstall Alfred CLI
    -v, --version   Install specific version (default: latest)
    
Examples:
    $0                    # Install latest version
    $0 -v v1.2.3         # Install specific version
    $0 --uninstall       # Uninstall Alfred CLI

Environment Variables:
    ALFRED_INSTALL_DIR   Custom installation directory (default: /usr/local/bin)
    
EOF
}

# Main installation function
main() {
    local version="$LATEST_VERSION"
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -u|--uninstall)
                uninstall
                ;;
            -v|--version)
                version="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # Use custom install directory if set
    if [[ -n "$ALFRED_INSTALL_DIR" ]]; then
        INSTALL_DIR="$ALFRED_INSTALL_DIR"
    fi
    
    log_info "Starting Alfred CLI installation..."
    
    # Security check
    check_root
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    log_info "Detected platform: $platform"
    
    # Get latest version if not specified
    if [[ "$version" == "latest" ]]; then
        version=$(get_latest_version)
        if [[ -z "$version" ]]; then
            log_error "Failed to get latest version"
            exit 1
        fi
    fi
    
    log_info "Installing version: $version"
    
    # Install binary
    install_binary "$platform" "$version"
    
    # Check PATH
    if ! check_path; then
        log_warning "Please update your PATH and restart your terminal"
    fi
    
    # Verify installation
    if verify_installation; then
        # Setup shell completion
        setup_completion
        
        echo ""
        log_success "🎉 Alfred CLI installation completed!"
        log_info "Get started with: alfred --help"
        log_info "Initialize a project: alfred init"
    else
        exit 1
    fi
}

# Run main function
main "$@"