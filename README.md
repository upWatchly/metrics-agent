# upWatchly metrics-agent

A lightweight host metrics collector for [upWatchly](https://upwatchly.com). It
runs on your server, samples system metrics, and reports them to the upWatchly
API. Single static Go binary, no runtime dependencies.

- **Linux** (amd64 / arm64) — systemd service or Docker container
- **Windows** (amd64) — native Windows service

---

## What it collects

Every report includes:

| Group | Details |
|-------|---------|
| **CPU** | Overall usage with user/system breakdown, core count |
| **Memory** | Used / total (virtual memory) |
| **Disk** | Used / total per mounted filesystem (fixed drives + volumes) |
| **Network** | In/out bytes per second (rate, computed between samples) |
| **Load average** | 1m / 5m / 15m — Linux only (Windows has no load average; reported as 0) |
| **Processes** | Top 50 by CPU + memory (name, pid, cpu%, mem%, RSS, user, command) |
| **Docker** | Running containers (name, image, state, ports) — when a Docker socket is reachable |
| **Host** | Hostname, OS, kernel/version, uptime, public IPv4/IPv6 |

Metrics are read through [`gopsutil`](https://github.com/shirou/gopsutil), which
uses the native OS facilities on each platform (procfs on Linux, WMI/PDH on
Windows) — so no `/host` mounts are needed on Windows.

---

## How it works

```
                 ┌──────────────────────────────────────────┐
                 │                 agent                     │
  collectLoop ──▶│  gopsutil sample ──▶ latest snapshot      │
  (background)   │                          │                │
                 │  reportLoop (ticker) ────┴──▶ POST report │──▶ upWatchly API
                 │                              ◀── response  │    /v1/servers/report
                 └──────────────────────────────────────────┘    (X-Server-Token: <UW_API_KEY>)
```

1. A background loop continuously samples metrics into an in-memory snapshot.
2. A report loop POSTs the latest snapshot to `POST {UW_API_ENDPOINT}/v1/servers/report`,
   authenticating with the `X-Server-Token` header (your `UW_API_KEY`).
3. The API response tells the agent its cadence:
   - **Normal mode** — reports every `reportInterval` seconds (default 60, clamped 10–3600).
   - **Live mode** — when someone is watching the server in the dashboard, the
     API flips the agent into fast reporting (1–60s). A background "slow
     collector" keeps refreshing the expensive metrics (disk, Docker) every few
     seconds so live reports stay cheap.
4. On the first report the API returns the server + organization identity, which
   the agent caches for subsequent reports.

The agent holds no local state or database — it is fully driven by environment
variables and the API responses.

---

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `UW_API_KEY` | **yes** | — | Server token, issued from the upWatchly dashboard |
| `UW_API_ENDPOINT` | no | `https://api.upwatchly.com` | API base URL |
| `UW_LOG_LEVEL` | no | `info` | `trace` / `debug` / `info` / `warn` / `error` |
| `UW_DISABLE_KEEP_ALIVE` | no | `false` | Disable HTTP keep-alive to the API |
| `UW_PPROF` | no | `false` | Expose Go pprof on `127.0.0.1:6060` (debugging only) |

---

## Installation

### Linux (systemd)

Installs the binary to `/usr/local/bin/upwatchly-agent` and registers a systemd
service:

```bash
curl -sSL https://github.com/upwatchly/metrics-agent/releases/latest/download/install.sh \
  | sudo bash -s -- <UW_API_KEY>
```

Re-running without an API key upgrades the binary in place and keeps your
existing configuration.

Manage the service:

```bash
systemctl status upwatchly-agent
journalctl -u upwatchly-agent -f          # logs
sudo systemctl restart upwatchly-agent
```

Change settings by editing the unit file and restarting:

```bash
sudo nano /etc/systemd/system/upwatchly-agent.service
sudo systemctl daemon-reload && sudo systemctl restart upwatchly-agent
```

Uninstall:

```bash
sudo systemctl stop upwatchly-agent
sudo systemctl disable upwatchly-agent
sudo rm /usr/local/bin/upwatchly-agent /etc/systemd/system/upwatchly-agent.service
```

### Docker (Linux)

The container reads host metrics from bind-mounted `/proc`, `/sys`, and `/etc`:

```bash
docker run -d --name upwatchly-agent --restart unless-stopped \
  -e UW_API_KEY=<UW_API_KEY> \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /etc:/host/etc:ro \
  ghcr.io/upwatchly/metrics-agent:latest
```

To also report Docker containers, mount the Docker socket:

```bash
  -v /var/run/docker.sock:/var/run/docker.sock:ro
```

### Windows (native service)

Run from an **elevated** PowerShell (Run as Administrator):

```powershell
irm https://github.com/upwatchly/metrics-agent/releases/latest/download/install.ps1 | iex
```

The installer downloads `upwatchly-agent.exe` to
`C:\Program Files\upwatchly-agent`, registers the **`UpwatchlyAgent`** Windows
service (runs as LocalSystem, auto-start, restart-on-failure), prompts for your
`UW_API_KEY`, and starts it. Service configuration is stored in the service's
registry key — the API key never touches the machine-wide environment.

Manage the service:

```powershell
Get-Service     UpwatchlyAgent
Restart-Service UpwatchlyAgent
Get-Content -Wait 'C:\ProgramData\upwatchly-agent\logs\metrics-agent.log'
```

Uninstall (elevated PowerShell):

```powershell
irm https://github.com/upwatchly/metrics-agent/releases/latest/download/uninstall.ps1 | iex
```

---

## Build from source

Requires Go 1.24+.

```bash
go build ./cmd/agent                 # current platform

# Cross-compile (static, no CGO)
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o upwatchly-agent-linux-amd64       ./cmd/agent
CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o upwatchly-agent-linux-arm64       ./cmd/agent
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o upwatchly-agent-windows-amd64.exe ./cmd/agent
```

Run it locally in the foreground:

```bash
UW_API_KEY=<key> UW_LOG_LEVEL=debug ./upwatchly-agent
```

On any platform the binary runs in the foreground when launched from a shell and
stops cleanly on `Ctrl-C` / `SIGTERM`. On Windows it additionally detects when
it is started by the Service Control Manager and runs under the service protocol.

---

## Releases & versioning

CI (`.github/workflows/ci.yml`) uses
[semantic-release](https://semantic-release.gitbook.io/) driven by
[Conventional Commits](https://www.conventionalcommits.org/):

- `fix:` → patch, `feat:` → minor, `feat!:` / `BREAKING CHANGE:` → major.
- On a release-worthy push to `main`, CI computes the next version, tags it,
  creates a GitHub Release with a generated `CHANGELOG.md`, then builds and
  publishes:
  - the Docker image `ghcr.io/upwatchly/metrics-agent:{version}` (and `:latest`),
  - Linux amd64/arm64 + Windows amd64 binaries and the install scripts, attached
    to the release.

The build version is stamped into the binary (`-ldflags -X main.version`) and
included in every metrics report.
