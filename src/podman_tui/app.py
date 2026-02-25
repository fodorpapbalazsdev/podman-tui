"""Main TUI application."""

from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.widgets import Footer, Header
from textual.app import App
from textual.binding import Binding

from .services.podman_service import PodmanService
from .services.config import Config
from .widgets.containers_pane import ContainersPane
from .widgets.logs_pane import LogsPane
from .widgets.system_df_pane import SystemDFPane


class PodmanTUI(App):
    """Main TUI application for Podman."""

    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("tab", "next_pane", "Next Pane"),
        Binding("shift+tab", "prev_pane", "Previous Pane"),
        Binding("f1", "focus_containers", "Containers"),
        Binding("f2", "focus_logs", "Logs"),
        Binding("f3", "focus_system_df", "System DF"),
        Binding("f4", "toggle_system_df", "Toggle System DF"),
        Binding("h", "show_help", "Help"),
    ]

    CSS = """
    Screen {
        layout: vertical;
    }

    Header {
        dock: top;
        height: 1;
    }

    Footer {
        dock: bottom;
        height: 1;
    }

    #main-container {
        height: 1fr;
    }

    #containers-pane {
        width: 1fr;
        height: 50%;
        border: solid $surface-lighten-2;
    }

    #logs-pane {
        width: 1fr;
        height: 50%;
        border: solid $surface-lighten-2;
    }

    #system-df-pane {
        width: 1fr;
        height: 1fr;
        border: solid $surface-lighten-2;
    }

    #containers-pane:focus-within,
    #logs-pane:focus-within,
    #system-df-pane:focus-within {
        border: solid $accent;
    }

    #containers-title,
    #logs-title {
        dock: top;
        height: 1;
        background: $boost;
        color: $text;
    }

    DataTable {
        height: 1fr;
    }

    RichLog {
        height: 1fr;
    }
    """

    def __init__(self):
        """Initialize the application."""
        super().__init__()
        self.config = Config()
        self.podman_service = PodmanService()
        self.panes = []
        self.current_pane_index = 0

    def compose(self) -> ComposeResult:
        """Compose the application."""
        yield Header(show_clock=True)

        with Vertical(id="main-container"):
            with Horizontal():
                with Vertical(id="containers-pane"):
                    yield ContainersPane(self.podman_service)

            with Horizontal():
                with Vertical(id="logs-pane"):
                    yield LogsPane(self.podman_service)

                with Vertical(id="system-df-pane"):
                    yield SystemDFPane(self.podman_service)

        yield Footer()

    def on_mount(self) -> None:
        """Mount the application."""
        self.title = "Podman TUI"
        self.sub_title = "Container Management Interface"

        # Collect panes
        self.panes = [
            self.query_one("#containers-pane"),
            self.query_one("#logs-pane"),
            self.query_one("#system-df-pane"),
        ]

        if self.panes:
            self._focus_pane(0)

    def _visible_panes(self) -> list:
        return [p for p in self.panes if p.display]

    def _focus_pane(self, index: int) -> None:
        self.current_pane_index = index
        container = self.panes[index]
        if index == 0:
            container.query_one(ContainersPane).focus_inner()
        elif index == 1:
            container.query_one(LogsPane).focus_inner()
        elif index == 2:
            container.query_one(SystemDFPane).focus_inner()

    def action_next_pane(self) -> None:
        """Move to next visible pane."""
        visible = self._visible_panes()
        if not visible:
            return
        try:
            current = visible.index(self.panes[self.current_pane_index])
            next_pane = visible[(current + 1) % len(visible)]
        except ValueError:
            next_pane = visible[0]
        self._focus_pane(self.panes.index(next_pane))

    def action_prev_pane(self) -> None:
        """Move to previous visible pane."""
        visible = self._visible_panes()
        if not visible:
            return
        try:
            current = visible.index(self.panes[self.current_pane_index])
            next_pane = visible[(current - 1) % len(visible)]
        except ValueError:
            next_pane = visible[-1]
        self._focus_pane(self.panes.index(next_pane))

    def action_focus_containers(self) -> None:
        """Focus the containers pane."""
        self._focus_pane(0)

    def action_focus_logs(self) -> None:
        """Focus the logs pane."""
        self._focus_pane(1)

    def action_focus_system_df(self) -> None:
        """Focus the system DF pane."""
        pane = self.panes[2]
        if pane.display:
            self._focus_pane(2)

    def action_toggle_system_df(self) -> None:
        """Toggle visibility of the system DF pane."""
        pane = self.panes[2]
        pane.display = not pane.display
        if not pane.display and self.current_pane_index == 2:
            self._focus_pane(0)

    async def on_containers_pane_show_logs(self, message: ContainersPane.ShowLogs) -> None:
        """Handle request to show logs for a container."""
        logs_pane = self.query_one(LogsPane)
        await logs_pane.set_container(message.container)

    def action_show_help(self) -> None:
        """Show help information."""
        self.notify("Help: Use Tab/Shift+Tab to navigate panes, q to quit")

