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
        border: solid $primary;
    }

    #logs-pane {
        width: 1fr;
        height: 50%;
        border: solid $primary;
    }

    #system-df-pane {
        width: 1fr;
        height: 1fr;
        border: solid $primary;
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
            self.panes[0].focus()

    def action_next_pane(self) -> None:
        """Move to next pane."""
        if not self.panes:
            return
        self.current_pane_index = (self.current_pane_index + 1) % len(self.panes)
        self.panes[self.current_pane_index].focus()

    def action_prev_pane(self) -> None:
        """Move to previous pane."""
        if not self.panes:
            return
        self.current_pane_index = (self.current_pane_index - 1) % len(self.panes)
        self.panes[self.current_pane_index].focus()

    def action_show_help(self) -> None:
        """Show help information."""
        self.notify("Help: Use Tab/Shift+Tab to navigate panes, q to quit")

