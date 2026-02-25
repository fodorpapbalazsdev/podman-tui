"""Configuration management."""

import os
from pathlib import Path
from typing import Any, Optional

import yaml


class Config:
    """Configuration manager."""

    def __init__(self, config_path: Optional[str] = None):
        """
        Initialize configuration.

        Args:
            config_path: Path to config file
        """
        self.config_path = Path(config_path) if config_path else self._get_default_path()
        self.config: dict[str, Any] = {}
        self.load()

    @staticmethod
    def _get_default_path() -> Path:
        """Get default config path."""
        config_home = os.getenv("XDG_CONFIG_HOME")
        if config_home:
            return Path(config_home) / "podman-tui" / "config.yaml"
        return Path.home() / ".config" / "podman-tui" / "config.yaml"

    def load(self) -> None:
        """Load configuration from file."""
        if self.config_path.exists():
            try:
                with open(self.config_path) as f:
                    self.config = yaml.safe_load(f) or {}
            except (yaml.YAMLError, IOError):
                self.config = {}
        else:
            self.config = self._get_defaults()

    def save(self) -> None:
        """Save configuration to file."""
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
        with open(self.config_path, "w") as f:
            yaml.dump(self.config, f, default_flow_style=False)

    @staticmethod
    def _get_defaults() -> dict[str, Any]:
        """Get default configuration."""
        return {
            "refresh_interval": 2,
            "log_lines": 100,
            "show_all_containers": False,
            "theme": "dark",
            "keybindings": {
                "quit": "q",
                "refresh": "r",
                "stop_container": "s",
                "start_container": "t",
                "pause_container": "p",
                "unpause_container": "u",
                "delete_container": "d",
                "next_pane": "tab",
                "prev_pane": "shift+tab",
                "select_up": "up",
                "select_down": "down",
                "help": "h",
            },
        }

    def get(self, key: str, default: Any = None) -> Any:
        """
        Get configuration value.

        Args:
            key: Configuration key (dot notation supported)
            default: Default value

        Returns:
            Configuration value
        """
        keys = key.split(".")
        value = self.config
        for k in keys:
            if isinstance(value, dict):
                value = value.get(k)
            else:
                return default
        return value if value is not None else default

    def set(self, key: str, value: Any) -> None:
        """
        Set configuration value.

        Args:
            key: Configuration key (dot notation supported)
            value: Value to set
        """
        keys = key.split(".")
        config = self.config
        for k in keys[:-1]:
            if k not in config:
                config[k] = {}
            config = config[k]
        config[keys[-1]] = value

