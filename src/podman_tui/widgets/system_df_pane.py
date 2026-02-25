"""System disk usage pane widget."""

from textual.widgets import Static

from ..models import SystemDFInfo
from ..services.podman_service import PodmanService


class SystemDFPane(Static):
    """Pane for displaying system disk usage."""

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
        """Render the pane."""
        self.load_df()

        if not self.df_info:
            return "Unable to load system disk usage information"

        content = f"""
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
        return content

    def load_df(self) -> None:
        """Load system disk usage information."""
        self.df_info = self.podman_service.get_system_df()

