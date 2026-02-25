# Podman TUI

A modern Terminal User Interface (TUI) for managing Podman containers, built with Python and Textual.

## Features

- 📦 **Container Management**: View, start, stop, pause, and unpause containers
- 📋 **Container Logs**: Real-time log viewing for selected containers
- 💾 **System Disk Usage**: Monitor disk usage with `podman system df`
- ⌨️ **Keyboard Navigation**: Full keyboard control with intuitive keybindings
- 🎨 **Multi-pane Layout**: Organized interface similar to k9s
- ⚙️ **Configurable**: YAML-based configuration system

## Requirements

- Python 3.10+
- Podman
- `uv` package manager

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd podman-tui

# Install with uv
uv sync

# Run the application
uv run podman-tui

