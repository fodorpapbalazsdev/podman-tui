"""Logs pane widget."""

from textual.app import ComposeResult
from textual.widgets import RichLog, Static
from textual.binding import Binding

from ..models import Container as ContainerModel, LogEntry
from ..services.podman_service import PodmanService


class LogsPane(Static):
    """Pane for displaying container logs."""

    BINDINGS = [
        Binding("c", "clear", "Clear"),
        Binding("r", "refresh", "Refresh"),
    ]

    def __init__(self, podman_service: PodmanService):
        """
        Initialize logs pane.

        Args:
            podman_service: Podman service instance
        """
        super().__init__()
        self.podman_service = podman_service
        self.current_container: ContainerModel | None = None
        self.logs: list[LogEntry] = []

    def compose(self) -> ComposeResult:
        """Compose the pane."""
        yield Static("Logs", id="logs-title")
        yield RichLog(id="logs-display")

    def set_container(self, container: ContainerModel) -> None:
        """
        Set the container to display logs for.

        Args:
            container: Container to display logs for
        """
        self.current_container = container
        self.load_logs()

    def load_logs(self) -> None:
        """Load logs from Podman."""
        if not self.current_container:
            return

        self.logs = self.podman_service.get_container_logs(
            self.current_container.id
        )
        self.display_logs()

    def display_logs(self) -> None:
        """Display logs in the widget."""
        log_display = self.query_one("#logs-display", RichLog)
        log_display.clear()

        if not self.current_container:
            log_display.write("No container selected")
            return

        if not self.logs:
            log_display.write("No logs available")
            return

        for log_entry in self.logs:
            log_display.write(str(log_entry))

    def action_clear(self) -> None:
        """Clear logs display."""
        log_display = self.query_one("#logs-display", RichLog)
        log_display.clear()

    def action_refresh(self) -> None:
        """Refresh logs."""
        self.load_logs()

