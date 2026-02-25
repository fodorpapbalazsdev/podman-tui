"""Entry point for the application."""

from .app import PodmanTUI


def main() -> None:
    """Run the application."""
    app = PodmanTUI()
    app.run()


if __name__ == "__main__":
    main()

