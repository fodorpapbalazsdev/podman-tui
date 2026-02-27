# podman-tui

A terminal UI for [Podman](https://podman.io/) built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Requirements

- Go 1.24+
- `podman` installed and on your `$PATH`

## Running

```bash
go run .
```

## Building

```bash
go build -o podman-tui .
./podman-tui
```

## Testing

```bash
go test ./...
```

Tests do not require a running Podman daemon — the test suite uses a mock that intercepts CLI calls internally.
