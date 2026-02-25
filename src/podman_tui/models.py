"""Data models for Podman TUI."""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Optional


class ContainerStatus(Enum):
    """Container status enumeration."""
    RUNNING = "running"
    EXITED = "exited"
    PAUSED = "paused"
    CREATED = "created"
    UNKNOWN = "unknown"


@dataclass
class Container:
    """Container data model."""
    id: str
    name: str
    status: ContainerStatus
    image: str
    created: datetime
    started: Optional[datetime] = None
    ports: list[Any] = field(default_factory=list)  # Changed to Any to support dicts
    memory_usage: Optional[str] = None
    cpu_usage: Optional[str] = None

    def __str__(self) -> str:
        """String representation."""
        return f"{self.name} ({self.id[:12]})"


@dataclass
class SystemDFInfo:
    """System disk usage information."""
    images_count: int
    images_size: str
    containers_count: int
    containers_size: str
    volumes_count: int
    volumes_size: str
    total_reclaimable: str
    timestamp: datetime = field(default_factory=datetime.now)

    def __str__(self) -> str:
        """String representation."""
        return f"Images: {self.images_count} ({self.images_size}) | Containers: {self.containers_count} ({self.containers_size}) | Volumes: {self.volumes_count} ({self.volumes_size})"


@dataclass
class LogEntry:
    """Log entry data model."""
    timestamp: datetime
    message: str
    level: str = "INFO"

    def __str__(self) -> str:
        """String representation."""
        return f"[{self.timestamp.strftime('%H:%M:%S')}] {self.level}: {self.message}"

