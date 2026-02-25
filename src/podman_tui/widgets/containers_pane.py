"""Containers pane widget."""

from textual.app import ComposeResult
from textual.widgets import DataTable, Static
from textual.binding import Binding

from ..models import Container as ContainerModel
from ..services.podman_service import PodmanService


class ContainersPane(Static):
    """Pane for displaying containers."""

    BINDINGS = [
        Binding("s", "stop", "Stop"),
        Binding("t", "start", "Start"),
        Binding("p", "pause", "Pause"),
        Binding("u", "unpause", "Unpause"),
        Binding("d", "delete", "Delete"),
        Binding("r", "refresh", "Refresh"),
    ]

    def __init__(self, podman_service: PodmanService):
        """
        Initialize containers pane.

        Args:
            podman_service: Podman service instance
        """
        super().__init__()
        self.podman_service = podman_service
        self.containers: list[ContainerModel] = []
        self.selected_container: ContainerModel | None = None

    def compose(self) -> ComposeResult:
        """Compose the pane."""
        yield Static("Containers", id="containers-title")
        yield DataTable(id="containers-table")

    def on_mount(self) -> None:
        """Mount the pane."""
        table = self.query_one("#containers-table", DataTable)
        table.cursor_type = "row"
        table.add_columns("Name", "ID", "Image", "Status", "Ports", "Memory", "CPU")
        self.load_containers()

    def load_containers(self) -> None:
        """Load containers from Podman."""
        self.containers = self.podman_service.get_containers(all=True)
        self._populate_table()

    def _populate_table(self) -> None:
        """Repopulate the table with current containers."""
        table = self.query_one("#containers-table", DataTable)
        table.clear()
        for container in self.containers:
            ports_str = self._format_ports(container.ports)
            table.add_row(
                container.name,
                container.id,
                container.image,
                container.status.value,
                ports_str,
                container.memory_usage or "-",
                container.cpu_usage or "-",
            )
        self.selected_container = self.containers[0] if self.containers else None

    def _format_ports(self, ports: list) -> str:
        """
        Format ports for display.

        Args:
            ports: List of port dictionaries or strings

        Returns:
            Formatted port string
        """
        if not ports:
            return "-"

        formatted_ports = []
        for port in ports:
            if isinstance(port, dict):
                # Handle port dictionary format
                host_ip = port.get("host_ip", "")
                host_port = port.get("host_port", "")
                container_port = port.get("container_port", "")
                protocol = port.get("protocol", "tcp")

                if host_port and container_port:
                    if host_ip:
                        formatted_ports.append(
                            f"{host_ip}:{host_port}->{container_port}/{protocol}"
                        )
                    else:
                        formatted_ports.append(
                            f"{host_port}->{container_port}/{protocol}"
                        )
                elif container_port:
                    formatted_ports.append(f"{container_port}/{protocol}")
            elif isinstance(port, str):
                # Handle string format
                formatted_ports.append(port)

        return ", ".join(formatted_ports) if formatted_ports else "-"

    def on_data_table_row_highlighted(self, event) -> None:
        """Handle cursor movement in the table."""
        if event.cursor_row < len(self.containers):
            self.selected_container = self.containers[event.cursor_row]

    def action_stop(self) -> None:
        """Stop selected container."""
        if self.selected_container:
            self.podman_service.stop_container(self.selected_container.id)
            self.load_containers()

    def action_start(self) -> None:
        """Start selected container."""
        if self.selected_container:
            self.podman_service.start_container(self.selected_container.id)
            self.load_containers()

    def action_pause(self) -> None:
        """Pause selected container."""
        if self.selected_container:
            self.podman_service.pause_container(self.selected_container.id)
            self.load_containers()

    def action_unpause(self) -> None:
        """Unpause selected container."""
        if self.selected_container:
            self.podman_service.unpause_container(self.selected_container.id)
            self.load_containers()

    def action_delete(self) -> None:
        """Delete selected container."""
        pass

    def action_refresh(self) -> None:
        """Refresh containers list."""
        self.load_containers()

