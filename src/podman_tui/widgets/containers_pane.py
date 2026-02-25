"""Containers pane widget."""

from textual.app import ComposeResult
from textual.containers import Container, Vertical
from textual.widgets import DataTable, Static
from textual.binding import Binding

from ..models import Container as ContainerModel
from ..services.podman_service import PodmanService


class ContainersPane(Static):
    """Pane for displaying containers."""

    BINDINGS = [
        Binding("s", "stop", "Stop", show=True),
        Binding("t", "start", "Start", show=True),
        Binding("p", "pause", "Pause", show=True),
        Binding("u", "unpause", "Unpause", show=True),
        Binding("d", "delete", "Delete", show=True),
        Binding("r", "refresh", "Refresh", show=True),
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
        self.selected_row: int = 0

    def compose(self) -> ComposeResult:
        """Compose the pane."""
        yield Static("Containers", id="containers-title")
        yield DataTable(id="containers-table")

    def on_mount(self) -> None:
        """Mount the pane."""
        self.load_containers()
        self.setup_table()

        # Focus the table
        table = self.query_one("#containers-table", DataTable)
        table.focus()

    def load_containers(self) -> None:
        """Load containers from Podman."""
        self.containers = self.podman_service.get_containers(all=True)

    def setup_table(self) -> None:
        """Setup the data table."""
        table = self.query_one("#containers-table", DataTable)

        # Clear existing rows
        table.clear()

        # Add columns
        table.add_columns(
            "Name",
            "ID",
            "Image",
            "Status",
            "Ports",
            "Memory",
            "CPU",
        )

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

    def on_data_table_row_selected(self, event) -> None:
        """Handle row selection in the table."""
        self.selected_row = event.cursor_row
        if event.cursor_row < len(self.containers):
            self.selected_container = self.containers[event.cursor_row]
            # Notify parent app about selection change
            self.post_message(self.ContainerSelected(self.selected_container))

    def _get_selected_container(self) -> ContainerModel | None:
        """
        Get the currently selected container.

        Returns:
            Selected container or None
        """
        table = self.query_one("#containers-table", DataTable)
        try:
            cursor_row = table.cursor_row
            if 0 <= cursor_row < len(self.containers):
                self.selected_container = self.containers[cursor_row]
                return self.selected_container
        except (IndexError, AttributeError):
            pass
        return self.selected_container

    def action_stop(self) -> None:
        """Stop selected container."""
        container = self._get_selected_container()
        if container:
            self.podman_service.stop_container(container.id)
            self.load_containers()
            self.setup_table()
            self.app.notify(f"Stopped container: {container.name}")

    def action_start(self) -> None:
        """Start selected container."""
        container = self._get_selected_container()
        if container:
            self.podman_service.start_container(container.id)
            self.load_containers()
            self.setup_table()
            self.app.notify(f"Started container: {container.name}")

    def action_pause(self) -> None:
        """Pause selected container."""
        container = self._get_selected_container()
        if container:
            self.podman_service.pause_container(container.id)
            self.load_containers()
            self.setup_table()
            self.app.notify(f"Paused container: {container.name}")

    def action_unpause(self) -> None:
        """Unpause selected container."""
        container = self._get_selected_container()
        if container:
            self.podman_service.unpause_container(container.id)
            self.load_containers()
            self.setup_table()
            self.app.notify(f"Unpaused container: {container.name}")

    def action_delete(self) -> None:
        """Delete selected container."""
        container = self._get_selected_container()
        if container:
            self.podman_service.remove_container(container.id, force=True)
            self.load_containers()
            self.setup_table()
            self.app.notify(f"Deleted container: {container.name}")

    def action_refresh(self) -> None:
        """Refresh containers list."""
        self.load_containers()
        self.setup_table()
        self.app.notify("Containers list refreshed")

    class ContainerSelected:
        """Message for container selection."""

        def __init__(self, container: ContainerModel):
            self.container = container
