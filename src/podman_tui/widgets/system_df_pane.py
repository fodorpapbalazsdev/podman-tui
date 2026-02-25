"""System disk usage pane widget."""

from textual.widgets import Static

from ..models import SystemDFInfo
from ..services.podman_service import PodmanService


class SystemDFPane(Static):
    """Pane for displaying system disk usage."""

    can_focus = True

    def focus_inner(self) -> None:
        """Focus this pane directly (no inner focusable child)."""
        self.focus()

    def __init__(self, podman_service: PodmanService):
        """
        Initialize system df pane.

        Args:
            podman_service: Podman service instance
        """
        super().__init__()
        self.podman_service = podman_service
        self.df_info: SystemDFInfo | None = None

    def render(self) -> str:
        """Render the pane using cached data (no subprocess calls here)."""
        if not self.df_info:
            return "Loading system disk usage..."

        return f"""
╔═══════════════════════════════════════════════════════════╗
║              System Disk Usage (podman system df)          ║
╚═══════════════════════════════════════════════════════════╝

📦 Images
   Count:  {self.df_info.images_count}
   Size:   {self.df_info.images_size}

📦 Containers
   Count:  {self.df_info.containers_count}
   Size:   {self.df_info.containers_size}

📦 Volumes
   Count:  {self.df_info.volumes_count}
   Size:   {self.df_info.volumes_size}

🔄 Reclaimable
   Size:   {self.df_info.total_reclaimable}

Last updated: {self.df_info.timestamp.strftime('%Y-%m-%d %H:%M:%S')}
"""

    async def on_mount(self) -> None:
        """Load disk usage data on mount."""
        await self.load_df()

    async def load_df(self) -> None:
        """Load system disk usage information asynchronously."""
        self.df_info = await self.podman_service.get_system_df()
        self.refresh()
