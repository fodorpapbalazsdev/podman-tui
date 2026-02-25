"""Podman service for executing podman commands."""

import json
import subprocess
from datetime import datetime
from typing import Optional

from ..models import Container, ContainerStatus, LogEntry, SystemDFInfo


class PodmanService:
    """Service for interacting with Podman."""

    def __init__(self):
        """Initialize Podman service."""
        self.podman_cmd = "podman"

    def _run_command(self, cmd: list[str]) -> tuple[str, str, int]:
        """
        Run a shell command.

        Args:
            cmd: Command and arguments as list

        Returns:
            Tuple of (stdout, stderr, returncode)
        """
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=10,
            )
            return result.stdout, result.stderr, result.returncode
        except subprocess.TimeoutExpired:
            return "", "Command timeout", 1
        except FileNotFoundError:
            return "", "Podman not found", 1

    def get_containers(self, all: bool = False) -> list[Container]:
        """
        Get list of containers.

        Args:
            all: Include stopped containers

        Returns:
            List of Container objects
        """
        cmd = [self.podman_cmd, "ps", "--format=json"]
        if all:
            cmd.insert(2, "-a")

        stdout, stderr, returncode = self._run_command(cmd)

        if returncode != 0:
            return []

        try:
            data = json.loads(stdout)
            containers = []
            for item in data:
                status = self._parse_status(item.get("State", "unknown"))
                container = Container(
                    id=item.get("Id", "")[:12],
                    name=item.get("Names", ["unknown"])[0],
                    status=status,
                    image=item.get("Image", "unknown"),
                    created=self._parse_datetime(item.get("Created", "")),
                    started=self._parse_datetime(item.get("StartedAt", "")),
                    ports=item.get("Ports", []) or [],
                    memory_usage=item.get("MemUsage", ""),
                    cpu_usage=item.get("CPUUsage", ""),
                )
                containers.append(container)
            return containers
        except json.JSONDecodeError:
            return []

    def stop_container(self, container_id: str) -> bool:
        """
        Stop a container.

        Args:
            container_id: Container ID or name

        Returns:
            True if successful
        """
        _, _, returncode = self._run_command(
            [self.podman_cmd, "stop", container_id]
        )
        return returncode == 0

    def start_container(self, container_id: str) -> bool:
        """
        Start a container.

        Args:
            container_id: Container ID or name

        Returns:
            True if successful
        """
        _, _, returncode = self._run_command(
            [self.podman_cmd, "start", container_id]
        )
        return returncode == 0

    def pause_container(self, container_id: str) -> bool:
        """
        Pause a container.

        Args:
            container_id: Container ID or name

        Returns:
            True if successful
        """
        _, _, returncode = self._run_command(
            [self.podman_cmd, "pause", container_id]
        )
        return returncode == 0

    def unpause_container(self, container_id: str) -> bool:
        """
        Unpause a container.

        Args:
            container_id: Container ID or name

        Returns:
            True if successful
        """
        _, _, returncode = self._run_command(
            [self.podman_cmd, "unpause", container_id]
        )
        return returncode == 0

    def remove_container(self, container_id: str, force: bool = False) -> bool:
        """
        Remove a container.

        Args:
            container_id: Container ID or name
            force: Force removal

        Returns:
            True if successful
        """
        cmd = [self.podman_cmd, "rm"]
        if force:
            cmd.append("-f")
        cmd.append(container_id)

        _, _, returncode = self._run_command(cmd)
        return returncode == 0

    def get_container_logs(
        self, container_id: str, lines: int = 100, follow: bool = False
    ) -> list[LogEntry]:
        """
        Get container logs.

        Args:
            container_id: Container ID or name
            lines: Number of lines to retrieve
            follow: Follow log output

        Returns:
            List of LogEntry objects
        """
        cmd = [
            self.podman_cmd,
            "logs",
            f"--tail={lines}",
            "--timestamps",
        ]
        if follow:
            cmd.append("-f")

        cmd.append(container_id)

        stdout, stderr, returncode = self._run_command(cmd)

        if returncode != 0:
            return [LogEntry(datetime.now(), f"Error: {stderr}", "ERROR")]

        logs = []
        for line in stdout.strip().split("\n"):
            if not line:
                continue
            try:
                # Parse timestamp and message
                parts = line.split(" ", 1)
                if len(parts) == 2:
                    timestamp_str, message = parts
                    timestamp = self._parse_datetime(timestamp_str)
                    logs.append(LogEntry(timestamp, message))
                else:
                    logs.append(LogEntry(datetime.now(), line))
            except (ValueError, IndexError):
                logs.append(LogEntry(datetime.now(), line))

        return logs

    def get_system_df(self) -> Optional[SystemDFInfo]:
        """
        Get system disk usage.

        Returns:
            SystemDFInfo object or None if error
        """
        cmd = [self.podman_cmd, "system", "df", "--format=json"]
        stdout, stderr, returncode = self._run_command(cmd)

        if returncode != 0:
            return None

        try:
            data = json.loads(stdout)

            # Handle list format (newer Podman versions)
            if isinstance(data, list):
                images_count = 0
                images_size = "0 B"
                containers_count = 0
                containers_size = "0 B"
                volumes_count = 0
                volumes_size = "0 B"
                total_reclaimable = "0 B"

                for item in data:
                    item_type = item.get("Type", "").lower()

                    if "image" in item_type:
                        images_count = item.get("Total", 0)
                        images_size = item.get("Size", "0 B")
                        total_reclaimable = item.get("Reclaimable", "0 B")

                    elif "container" in item_type:
                        containers_count = item.get("Total", 0)
                        containers_size = item.get("Size", "0 B")

                    elif "volume" in item_type:
                        volumes_count = item.get("Total", 0)
                        volumes_size = item.get("Size", "0 B")

                return SystemDFInfo(
                    images_count=images_count,
                    images_size=images_size,
                    containers_count=containers_count,
                    containers_size=containers_size,
                    volumes_count=volumes_count,
                    volumes_size=volumes_size,
                    total_reclaimable=total_reclaimable,
                )

            # Handle dict format (older Podman versions)
            else:
                return SystemDFInfo(
                    images_count=len(data.get("Images", [])),
                    images_size=self._format_size(
                        sum(img.get("Size", 0) for img in data.get("Images", []))
                    ),
                    containers_count=len(data.get("Containers", [])),
                    containers_size=self._format_size(
                        sum(
                            cnt.get("RwSize", 0)
                            for cnt in data.get("Containers", [])
                        )
                    ),
                    volumes_count=len(data.get("Volumes", [])),
                    volumes_size=self._format_size(
                        sum(
                            vol.get("UsageData", {}).get("Size", 0)
                            for vol in data.get("Volumes", [])
                        )
                    ),
                    total_reclaimable=self._format_size(
                        data.get("ReclaimableSize", 0)
                    ),
                )
        except (json.JSONDecodeError, KeyError, TypeError, AttributeError):
            return None

    @staticmethod
    def _parse_status(status_str: str) -> ContainerStatus:
        """
        Parse container status string.

        Args:
            status_str: Status string from Podman

        Returns:
            ContainerStatus enum value
        """
        if not status_str:
            return ContainerStatus.UNKNOWN

        status_lower = status_str.lower()
        if "running" in status_lower:
            return ContainerStatus.RUNNING
        elif "exited" in status_lower:
            return ContainerStatus.EXITED
        elif "paused" in status_lower:
            return ContainerStatus.PAUSED
        elif "created" in status_lower:
            return ContainerStatus.CREATED
        return ContainerStatus.UNKNOWN

    @staticmethod
    def _parse_datetime(dt_str: str) -> datetime:
        """
        Parse datetime string from Podman.

        Handles multiple formats:
        - Unix timestamp (seconds): 1771967664
        - ISO format: 2024-02-25T10:30:45.123456789Z
        - ISO format with timezone: 2024-02-25T10:30:45+00:00

        Args:
            dt_str: Datetime string to parse

        Returns:
            datetime object
        """
        if not dt_str or dt_str == "0":
            return datetime.now()

        # Try parsing as Unix timestamp (integer)
        try:
            timestamp = float(dt_str)
            # Check if it's a reasonable Unix timestamp (between 2000 and 2100)
            if 946684800 < timestamp < 4102444800:
                return datetime.fromtimestamp(timestamp)
        except (ValueError, OSError, OverflowError):
            pass

        # Try parsing as ISO format with Z suffix
        try:
            if dt_str.endswith("Z"):
                return datetime.fromisoformat(dt_str.replace("Z", "+00:00"))
        except ValueError:
            pass

        # Try parsing as ISO format with timezone
        try:
            return datetime.fromisoformat(dt_str)
        except ValueError:
            pass

        # Try parsing as ISO format without timezone
        try:
            return datetime.fromisoformat(dt_str.split("+")[0].split("Z")[0])
        except ValueError:
            pass

        # If all parsing fails, return current time
        return datetime.now()

    @staticmethod
    def _format_size(size_bytes: int) -> str:
        """
        Format bytes to human readable size.

        Args:
            size_bytes: Size in bytes

        Returns:
            Formatted size string
        """
        if size_bytes == 0:
            return "0 B"

        size_bytes = float(size_bytes)
        for unit in ["B", "KB", "MB", "GB"]:
            if size_bytes < 1024:
                return f"{size_bytes:.1f} {unit}"
            size_bytes /= 1024
        return f"{size_bytes:.1f} TB"

