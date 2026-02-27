package podman

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fpbpi/podman-tui/internal/models"
)

// ---- mock "podman" binary via the helper-process pattern ----
//
// When GO_PODMAN_MOCK=1 is set the test binary acts as a fake podman binary.
// Tests set that env var (via t.Setenv) and point Service.podmanCmd at os.Args[0]
// so the service executes the test binary itself, which re-enters here and
// produces canned JSON output.

const (
	mockContainersJSON = `[` +
		`{"Id":"abc123def456ghi789","Names":["/web"],"Image":"nginx:latest","State":"running","Created":1700000000,"StartedAt":1700000100,` +
		`"Ports":[{"host_ip":"0.0.0.0","host_port":80,"container_port":80,"protocol":"tcp"}]},` +
		`{"Id":"def456ghi789jkl012","Names":["/db"],"Image":"postgres:15","State":"exited","Created":1699900000,"StartedAt":1699900100,"Ports":[]}` +
		`]`

	mockStatsJSON = `[{"id":"abc123def456","mem_usage":"50MiB / 2GiB","cpu_percent":"0.50%"}]`

	// Two timestamped log lines separated by a newline (no trailing newline so
	// we can control exactly what stdout contains in the mock).
	mockLogsOutput = "2024-01-02T15:04:05.000000000Z Hello world\n2024-01-02T15:04:06.000000000Z Error: something"

	mockSystemDFJSON = `[` +
		`{"Type":"Images","Total":4,"Size":"294.3MB","RawReclaimable":110007591},` +
		`{"Type":"Containers","Total":2,"Size":"1.2MB","RawReclaimable":0},` +
		`{"Type":"Volumes","Total":1,"Size":"100MB","RawReclaimable":52428800}` +
		`]`
)

// TestMain intercepts test-binary invocations used as the fake podman binary.
func TestMain(m *testing.M) {
	if os.Getenv("GO_PODMAN_MOCK") == "1" {
		runMockPodman()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runMockPodman acts as a minimal podman stub. It reads os.Args to decide what
// JSON/text to emit, and honours GO_PODMAN_MOCK_FAIL to simulate failures.
func runMockPodman() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}

	sub := os.Args[1]

	if os.Getenv("GO_PODMAN_MOCK_FAIL") == sub {
		fmt.Fprintln(os.Stderr, "mock: command failed")
		os.Exit(1)
	}

	switch sub {
	case "ps":
		fmt.Println(mockContainersJSON)
	case "stats":
		fmt.Println(mockStatsJSON)
	case "logs":
		// The container ID is always the last argument.
		id := os.Args[len(os.Args)-1]
		if id != "empty" {
			// Use Print, not Println, to avoid adding an extra trailing newline
			// that would result in a spurious empty log entry.
			fmt.Print(mockLogsOutput)
		}
	case "system":
		if len(os.Args) > 2 && os.Args[2] == "df" {
			fmt.Println(mockSystemDFJSON)
		}
	case "start", "stop", "pause", "unpause", "rm":
		// success – no output needed
	default:
		fmt.Fprintln(os.Stderr, "mock: unknown subcommand: "+sub)
		os.Exit(1)
	}
}

// mockService returns a Service whose podmanCmd points to the test binary
// itself (acting as the fake podman) and whose inherited environment
// activates the mock mode.
func mockService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("GO_PODMAN_MOCK", "1")
	return &Service{podmanCmd: os.Args[0]}
}

// ---- formatBytes ----

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{110007591, "104.9 MB"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, formatBytes(tc.in), "formatBytes(%d)", tc.in)
	}
}

// ---- GetContainers ----

func TestGetContainers_WithStats(t *testing.T) {
	cons, err := mockService(t).GetContainers(true, true)
	require.NoError(t, err)
	require.Len(t, cons, 2)

	web := cons[0]
	assert.Equal(t, "web", web.Name)
	assert.Equal(t, models.StatusRunning, web.Status)
	assert.Equal(t, "50MiB / 2GiB", web.MemoryUsage)
	assert.Equal(t, "0.50%", web.CPUUsage)
	require.Len(t, web.Ports, 1)
	assert.Equal(t, "80", web.Ports[0].HostPort)
	assert.Equal(t, "80", web.Ports[0].ContainerPort)

	db := cons[1]
	assert.Equal(t, models.StatusExited, db.Status)
	// "db" is not in the stats map so it should get the default placeholder.
	assert.Equal(t, "-", db.MemoryUsage)
	assert.Equal(t, "-", db.CPUUsage)
}

func TestGetContainers_WithoutStats(t *testing.T) {
	cons, err := mockService(t).GetContainers(true, false)
	require.NoError(t, err)
	for _, c := range cons {
		assert.Equal(t, "-", c.MemoryUsage, "container %q", c.Name)
		assert.Equal(t, "-", c.CPUUsage, "container %q", c.Name)
	}
}

func TestGetContainers_NameTrimPrefix(t *testing.T) {
	// The mock JSON uses "/web" – the leading slash must be stripped.
	cons, err := mockService(t).GetContainers(true, false)
	require.NoError(t, err)
	assert.Equal(t, "web", cons[0].Name)
}

func TestGetContainers_PSFailure(t *testing.T) {
	svc := mockService(t)
	t.Setenv("GO_PODMAN_MOCK_FAIL", "ps")
	_, err := svc.GetContainers(true, false)
	require.Error(t, err)
}

// ---- GetContainerLogs ----

func TestGetContainerLogs_Success(t *testing.T) {
	logs, err := mockService(t).GetContainerLogs("some-container-id", 200)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "Hello world", logs[0].Message)
	assert.False(t, logs[0].Timestamp.IsZero(), "first entry should have a timestamp")
	assert.Equal(t, "Error: something", logs[1].Message)
}

func TestGetContainerLogs_Empty(t *testing.T) {
	// Container ID "empty" triggers the mock to produce no output.
	logs, err := mockService(t).GetContainerLogs("empty", 200)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestGetContainerLogs_Failure(t *testing.T) {
	svc := mockService(t)
	t.Setenv("GO_PODMAN_MOCK_FAIL", "logs")
	_, err := svc.GetContainerLogs("some-id", 200)
	require.Error(t, err)
}

// ---- GetSystemDF ----

func TestGetSystemDF(t *testing.T) {
	info, err := mockService(t).GetSystemDF()
	require.NoError(t, err)
	assert.Equal(t, 4, info.ImagesCount)
	assert.Equal(t, "294.3MB", info.ImagesSize)
	assert.Equal(t, 2, info.ContainersCount)
	assert.Equal(t, 1, info.VolumesCount)
	// TotalReclaimable = formatBytes(110007591 + 0 + 52428800) = formatBytes(162436391) = "154.9 MB"
	assert.Equal(t, "154.9 MB", info.TotalReclaimable)
}

func TestGetSystemDF_Failure(t *testing.T) {
	svc := mockService(t)
	t.Setenv("GO_PODMAN_MOCK_FAIL", "system")
	_, err := svc.GetSystemDF()
	require.Error(t, err)
}

// ---- container lifecycle actions ----

func TestStartContainer(t *testing.T) {
	assert.NoError(t, mockService(t).StartContainer("abc123"))
}

func TestStartContainer_Failure(t *testing.T) {
	svc := mockService(t)
	t.Setenv("GO_PODMAN_MOCK_FAIL", "start")
	assert.Error(t, svc.StartContainer("abc123"))
}

func TestStopContainer(t *testing.T) {
	assert.NoError(t, mockService(t).StopContainer("abc123"))
}

func TestPauseContainer(t *testing.T) {
	assert.NoError(t, mockService(t).PauseContainer("abc123"))
}

func TestUnpauseContainer(t *testing.T) {
	assert.NoError(t, mockService(t).UnpauseContainer("abc123"))
}

func TestRemoveContainer(t *testing.T) {
	assert.NoError(t, mockService(t).RemoveContainer("abc123", false))
}

func TestRemoveContainer_Force(t *testing.T) {
	assert.NoError(t, mockService(t).RemoveContainer("abc123", true))
}
