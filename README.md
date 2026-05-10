# V1 File Security Scanner

A malware-scanning tool powered by the **Trend Vision One File Security SDK**. Browse folders, scan for threats, and download PDF reports — all through a clean web interface. No technical knowledge required.

Available in four deployment options:

| Option | Platform | UI | Requires |
|--------|----------|----|----------|
| [Docker](#option-1-docker) | Any OS | Browser at `localhost:8080` | Docker |
| [macOS App](#option-2-macos-app) | macOS 12+ | Opens your browser automatically | Nothing |
| [Linux binary](#option-3-linux-binary) | Linux x86-64 | Browser (URL printed to terminal) | Nothing |
| [Windows app](#option-4-windows-app) | Windows 10/11 | Dedicated app window (Edge app mode) | Nothing |

---

## Option 1 — Docker

Best for: scanning drives mounted from any OS, running on a server, or when you don't want to install anything locally.

### Start

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Then open **http://localhost:8080** in your browser.

### Stop / Restart

```bash
docker stop v1fs-scanner
docker restart v1fs-scanner
```

### Scan external drives or folders from your host

Mount the path you want to scan when starting the container:

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /path/to/drive:/mnt/drive:ro \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Then in the app browse to `/mnt/drive`.

> **Windows users:** BitLocker-encrypted drives must be unlocked in Windows first. Once unlocked, they appear as normal directories and can be mapped with `-v`.

---

## Option 2 — macOS App

Best for: scanning your Mac directly without Docker. The script builds the binary on first run and opens your default browser automatically.

### Run

```bash
cd apps/macos
./run.sh
```

The script detects your architecture (Intel or Apple Silicon), compiles the binary if needed, and opens `http://localhost:8080` in your browser. Config and reports are saved to `~/Library/Application Support/V1FSScanner/`.

### Build a .app bundle (optional)

```bash
make darwin-app
open apps/macos/V1FSScanner.app
```

> First launch may require: **System Settings → Privacy & Security → Open Anyway** (Gatekeeper prompt for unsigned binaries).
> Or clear the quarantine flag first: `xattr -cr apps/macos/V1FSScanner.app`

---

## Option 3 — Linux Binary

Best for: servers, NAS devices, or automated scanning pipelines. Runs headlessly — no browser is opened, the URL is printed to the terminal.

### Run

```bash
cd apps/linux
./run.sh
```

Output:

```
V1FS Scanner ready → http://localhost:8080
```

Open that URL in any browser. Config and reports are saved to `~/.config/V1FSScanner/`.

### Change the port

```bash
PORT=9000 ./v1fs-scanner
```

### Run as a systemd service

```ini
[Unit]
Description=V1FS Scanner
After=network.target

[Service]
ExecStart=/opt/v1fs-scanner/v1fs-scanner
Environment=PORT=8080
Environment=TM_V1_API_KEY=your-key-here
Environment=TM_V1_REGION=us-east-1
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

---

## Option 4 — Windows App

Best for: scanning Windows machines natively. The `.exe` has no console window and opens a dedicated **Microsoft Edge app-mode window** (no browser address bar or tabs — looks and feels like a native desktop app). Falls back to Chrome app-mode, then the default browser if neither is found.

### Build

On any machine with Go installed (cross-compilation works from macOS or Linux too):

```bash
make windows
```

Output: `apps/windows/v1fs-scanner.exe`

### Run

Copy `apps/windows/v1fs-scanner.exe` to the Windows machine and double-click it. A window opens automatically pointing to `http://localhost:8080`. Config and reports are saved to `%APPDATA%\V1FSScanner\`.

> Edge is always available on Windows 10/11. If you prefer Chrome, have it installed and it will be used as the second choice.

### Scanning drives

The app browser shows all accessible drive letters (C:, D:, E:, …). Navigate to the drive you want to scan and select folders normally.

---

## Getting Started (all platforms)

### 1. Configure your API key

1. Click **Settings** in the left menu
2. Under **V1 File Security settings**, enter your **Trend Vision One API key** and **Region**
3. Click **Save scanner settings**

### 2. Select folders to scan

1. Click **Scanner**
2. Browse drives and folders in the **Locations** panel
3. Tick the folders you want to scan — you can add multiple

### 3. Start the scan

1. Click **Start scan**
2. Optionally give the scan a name
3. Watch live progress: files scanned, detections, throughput

### 4. Download the report

Click **Download PDF report** when the scan finishes. The report includes:
- All scanned paths
- Malicious files with names and malware labels
- Optional SHA-256 hashes
- Scan tags you configured
- Clean file list (if report mode is set to "all")

---

## Settings Overview

### Scanner Connection

| Setting | Description |
|---------|-------------|
| API Key | Your Trend Vision One credentials |
| Region | Where your API key is registered |
| Scanner type | SaaS (cloud) or Local (on-premises gRPC) |

### When Malware is Detected

| Action | Effect |
|--------|--------|
| Log only | Record in report, keep file |
| Quarantine | Move file to an isolated folder |
| Delete | Permanently remove the file |

### Advanced Options

| Option | Default |
|--------|---------|
| SHA-256 hashes | Off (slows scans on large drives) |
| Predictive machine learning | Off |
| Max simultaneous scans | 8 (up to 10,000) |
| Report mode | Malicious files only |

### Scan Tags

Add custom labels (e.g., `project-x`, `usb-drive`) that appear in PDF reports and are forwarded to Vision One for filtering. Up to 32 tags.

---

## Testing

Before scanning real data, verify your setup:

1. Click **Settings → Test options**
2. **Test scanner connection** — confirms your API key and region work
3. **Submit EICAR test file** — confirms malware detection works
4. **Submit clean test file** — confirms clean files are not flagged

---

## Scan History

Click **History** to review all completed scans with date, paths, file count, detections, and PDF download links.

---

## Troubleshooting

### "Scan connection failed"
Check your API key and region in **Settings**, then click **Test scanner connection**.

### Docker: only finds ~300 files when scanning "/"
You are scanning inside the container, not your real disk. Mount your host path:

```bash
docker run ... -v /Users/you:/mnt/home:ro ...
```

Then scan `/mnt/home`.

### macOS: "app is damaged" / Gatekeeper blocks the binary
The binary is not Apple-notarized. Allow it via:
```
System Settings → Privacy & Security → Open Anyway
```
Or from Terminal:
```bash
xattr -cr ./v1fs-scanner   # clear quarantine flag
```

### Windows: nothing opens after double-click
Edge may be in a non-standard install location. Open a browser manually at `http://localhost:8080`. Check Task Manager — the process should be running.

---

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `TM_V1_API_KEY` | Vision One API key (avoids entering in UI) | — |
| `TM_V1_REGION` | API region | — |
| `PORT` | HTTP port | `8080` |
| `V1FS_CONFIG_PATH` | Full path to config file | OS-specific |
| `V1FS_REPORTS_DIR` | Directory for PDF reports | OS-specific |

Default data directories by platform:

| Platform | Config & reports location |
|----------|--------------------------|
| Docker | `/data/` (persistent volume) |
| macOS | `~/Library/Application Support/V1FSScanner/` |
| Linux | `~/.config/V1FSScanner/` |
| Windows | `%APPDATA%\V1FSScanner\` |

---

## Project Structure

```
apps/
  macos/
    run.sh              ← build + launch (macOS)
    v1fs-scanner        ← compiled binary (git-ignored, created by run.sh)
    V1FSScanner.app/    ← .app bundle   (git-ignored, created by make darwin-app)
  linux/
    run.sh              ← build + launch (Linux)
    v1fs-scanner        ← compiled binary (git-ignored, created by run.sh)
  windows/
    v1fs-scanner.exe    ← compiled binary (git-ignored, created by make windows)
scripts/
  build-macos-app.sh    ← wraps binary in a .app bundle
web/                    ← UI assets (embedded into binary at build time)
internal/               ← Go source
Makefile                ← explicit build targets for all platforms
Dockerfile              ← containerised build
```

## Build from Source (Developers)

Requires **Go 1.24+** and `make`.

```bash
git clone https://github.com/yourusername/tool-v1fs-manual-scan.git
cd tool-v1fs-manual-scan

make docker          # Docker image
make darwin-arm64    # macOS Apple Silicon → apps/macos/v1fs-scanner
make darwin          # macOS Intel        → apps/macos/v1fs-scanner
make darwin-app      # macOS .app bundle  → apps/macos/V1FSScanner.app
make linux           # Linux x86-64       → apps/linux/v1fs-scanner
make windows         # Windows x86-64     → apps/windows/v1fs-scanner.exe
make all             # Docker + all native binaries
make run             # Run locally for development
```

Web assets are embedded in the binary at build time — no separate `web/` folder is needed at runtime.

---

## Supported Regions

- `us-east-1` — United States East
- `eu-central-1` — Europe Central
- `eu-west-2` — Europe West
- `ca-central-1` — Canada
- `ap-southeast-1` — Asia Pacific Southeast
- `ap-southeast-2` — Asia Pacific Southeast 2
- `ap-northeast-1` — Asia Pacific Northeast
- `ap-south-1` — Asia Pacific South
- `me-central-1` — Middle East Central

---

## Key Features

- **Four deployment options** — Docker, macOS .app, Linux binary, Windows .exe
- **Self-contained binaries** — web UI embedded, no external files needed at runtime
- **Web interface** — no command line required for day-to-day use
- **Real-time progress** — live file counters, throughput, ETA
- **PDF reports** — malware details, file hashes, scan tags, clean-file list
- **Scan history** — review and re-download past reports
- **Multiple scan paths** — up to 32 folders per job
- **Scan tags** — custom labels forwarded to Vision One
- **Custom malware actions** — log, quarantine, or delete
- **Persistent config** — settings survive restarts on all platforms
- **EICAR test support** — built-in verification before scanning real data

---

Powered by [Trend Vision One™ File Security SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk)
