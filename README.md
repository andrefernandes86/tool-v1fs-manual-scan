# V1 File Security Scanner

A malware-scanning tool powered by the **Trend Vision One File Security SDK**. Browse folders, scan for threats, and download PDF reports — all through a clean web interface. No command-line knowledge required for day-to-day use.

Available in four deployment options:

| Option | Platform | UI | Requires |
|--------|----------|----|----------|
| [Docker](#option-1-docker) | Any OS | Browser at `localhost:8080` | Docker Desktop |
| [macOS App](#option-2-macos-app) | macOS 12+ | Opens your browser automatically | Go 1.24+ (first run only) |
| [Linux Binary](#option-3-linux-binary) | Linux x86-64 | Browser (URL printed to terminal) | Go 1.24+ (first run only) |
| [Windows App](#option-4-windows-app) | Windows 10/11 | Dedicated app window | Nothing |

---

## Option 1 — Docker

Best for: scanning drives mounted from any OS, running on a server, or when you prefer not to install anything locally. All data (config, reports) persists in a named volume.

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) installed and running.

### Build the image

```bash
git clone https://github.com/andrefernandes86/tool-v1fs-manual-scan.git
cd tool-v1fs-manual-scan
docker build -t v1fs-scanner .
```

### Start the container

```bash
docker run -d \
  -p 8080:8080 \
  -v v1fs-data:/data \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Then open **http://localhost:8080** in your browser.

| Flag | Purpose |
|------|---------|
| `-p 8080:8080` | Expose the web UI on your host at port 8080 |
| `-v v1fs-data:/data` | Persist config and PDF reports across restarts |
| `--name v1fs-scanner` | Give the container a memorable name |

### Scan a folder or drive on your host

Mount the path you want to scan when starting the container:

```bash
docker run -d \
  -p 8080:8080 \
  -v v1fs-data:/data \
  -v /path/to/folder:/mnt/target:ro \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Then in the app navigate to `/mnt/target` and start your scan.

**Examples:**

```bash
# Scan your entire home directory (macOS/Linux)
-v /Users/yourname:/mnt/home:ro

# Scan a USB drive on macOS
-v /Volumes/MyUSB:/mnt/usb:ro

# Scan a folder on Windows (PowerShell)
-v C:\Users\you\Documents:/mnt/docs:ro
```

> **Windows + BitLocker:** Encrypted drives must be unlocked in Windows first. Once unlocked, the drive letter appears as a normal directory and can be mounted with `-v`.

### Stop and restart

```bash
docker stop v1fs-scanner
docker start v1fs-scanner
```

### Remove the container (keeps your data volume)

```bash
docker rm v1fs-scanner
```

### Change the port

```bash
docker run -d -p 9000:8080 -v v1fs-data:/data --name v1fs-scanner v1fs-scanner:latest
```

Then open **http://localhost:9000**.

---

## Option 2 — macOS App

Best for: scanning your Mac directly without Docker. The script builds the binary on first run and opens your browser automatically. No dependencies needed after that.

### Prerequisites

- **Go 1.24+** — install via [go.dev/dl](https://go.dev/dl/) or `brew install go`.
- Only needed the very first time (or after a code update). Once built, the binary runs standalone.

### Run

```bash
git clone https://github.com/andrefernandes86/tool-v1fs-manual-scan.git
cd tool-v1fs-manual-scan/apps/macos
./run.sh
```

The script:
1. Detects your architecture (Intel or Apple Silicon)
2. Compiles the binary if it doesn't exist yet (takes ~30 seconds first time)
3. Starts the server and opens `http://localhost:8080` in Chrome, Firefox, or your default browser

Config and PDF reports are saved to:
```
~/Library/Application Support/V1FSScanner/
```

### Subsequent runs

```bash
cd apps/macos
./run.sh
```

The binary is already compiled — it starts in under a second.

### Change the port

```bash
PORT=9000 ./run.sh
```

### Build a .app bundle (optional — double-click to open)

```bash
make darwin-app
open apps/macos/V1FSScanner.app
```

> **Gatekeeper prompt:** Because the binary is not Apple-notarized, macOS may block it on first launch.
> Go to **System Settings → Privacy & Security → Open Anyway**, or run:
> ```bash
> xattr -cr apps/macos/V1FSScanner.app
> ```

---

## Option 3 — Linux Binary

Best for: servers, NAS devices, headless VMs, or automated scanning pipelines. No browser is opened — the URL is printed to the terminal.

### Prerequisites

- **Go 1.24+** — install via [go.dev/dl](https://go.dev/dl/) or your package manager.
- Only needed the very first time (or after a code update).

### Run

```bash
git clone https://github.com/andrefernandes86/tool-v1fs-manual-scan.git
cd tool-v1fs-manual-scan/apps/linux
./run.sh
```

Output:

```
V1FS Scanner ready → http://localhost:8080
```

Open that URL in any browser. Config and PDF reports are saved to:
```
~/.config/V1FSScanner/
```

### Change the port

```bash
PORT=9000 ./run.sh
```

### Pass your API key via environment variable (skip entering it in the UI)

```bash
TM_V1_API_KEY=your-api-key TM_V1_REGION=us-east-1 ./v1fs-scanner
```

### Run as a systemd service (auto-start on boot)

Create `/etc/systemd/system/v1fs-scanner.service`:

```ini
[Unit]
Description=V1 File Security Scanner
After=network.target

[Service]
ExecStart=/opt/v1fs-scanner/v1fs-scanner
Environment=PORT=8080
Restart=on-failure
User=youruser

[Install]
WantedBy=multi-user.target
```

Then enable it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now v1fs-scanner
```

---

## Option 4 — Windows App

Best for: scanning Windows machines natively. The `.exe` opens a dedicated **Microsoft Edge app-mode window** — no browser chrome, no address bar, feels like a native desktop app. Falls back to Chrome app-mode, then the default browser.

> **AV note:** The Windows binary contains no malware test file bytes. The built-in malware test is not available on Windows — use **Test scanner connection** to verify your API key instead.

### Download (pre-built — no install needed)

Download the latest `v1fs-scanner.exe` from the [Releases page](https://github.com/andrefernandes86/tool-v1fs-manual-scan/releases/latest), copy it to your Windows machine, and double-click it. Nothing else to install.

### Build it yourself

On any machine with Go 1.24+ and `make`:

```bash
git clone https://github.com/andrefernandes86/tool-v1fs-manual-scan.git
cd tool-v1fs-manual-scan
make windows
```

Output: `apps/windows/v1fs-scanner.exe`

Cross-compilation works from macOS or Linux — no Windows machine required to build.

### Run

1. Copy `apps/windows/v1fs-scanner.exe` to your Windows machine.
2. Double-click it.
3. A dedicated app window opens automatically at `http://localhost:8080`.

Config and PDF reports are saved to:
```
%APPDATA%\V1FSScanner\
```
(e.g. `C:\Users\YourName\AppData\Roaming\V1FSScanner\`)

### Change the port

Open a Command Prompt or PowerShell and run:

```powershell
$env:PORT=9000; .\v1fs-scanner.exe
```

### Scanning drives

All accessible drive letters (C:, D:, E:, …) appear in the folder browser. Navigate to the drive and select folders normally.

### Nothing opens after double-click?

- Check **Task Manager** — `v1fs-scanner.exe` should be in the list.
- If the process is running but no window appeared, open `http://localhost:8080` manually in any browser.
- Edge is always available on Windows 10/11. If you prefer Chrome, install it and it will be detected automatically.

---

## First-Time Setup (all platforms)

### 1. Configure your API key

1. Click **Settings** in the left menu
2. Under **V1 File Security settings**, enter your **Trend Vision One API key** and **Region**
3. Click **Save scanner settings**

Need an API key? Log in to [Vision One](https://portal.trendmicro.com) → **Administration → API Keys**.

### 2. Test your connection

1. Still in **Settings**, scroll to **Test options**
2. Click **Test scanner connection** — you should see a green "connection OK" message
3. (macOS/Linux only) Click **Submit malware test file** to confirm malware detection works

### 3. Select folders to scan

1. Click **Scanner** in the left menu
2. Use the **Locations** panel to browse to the folder(s) you want to scan
3. Tick each folder — you can queue up to 32 paths per job

### 4. Start the scan

1. Click **Start scan**
2. Optionally give the scan a name (helps find it in History later)
3. Watch live progress: files scanned, detections, throughput, ETA

### 5. Download the report

When the scan finishes, click **Download PDF report**. The report includes:
- All scanned paths and total file count
- Malicious files with names and threat labels
- Optional SHA-256 hashes of malicious files
- Scan tags you configured
- Full clean-file list (if **Report mode** is set to "All files")

---

## Settings Reference

### Scanner Connection

| Setting | Description |
|---------|-------------|
| API Key | Your Trend Vision One API key |
| Region | The region your key is registered in (see [Supported Regions](#supported-regions)) |
| Scanner type | **SaaS** (Trend Cloud) or **Local** (on-premises gRPC gateway) |
| Local scanner URL | gRPC address of your on-premises gateway (Local mode only) |

### When Malware is Detected

| Action | Effect |
|--------|--------|
| Log only | Record in report, leave file in place |
| Move to quarantine | Move file to an isolated folder you specify |
| Delete | Permanently remove the file |

### Advanced Options

| Option | Default | Notes |
|--------|---------|-------|
| SHA-256 hashes | Off | Enables per-file hashing for malicious files; slows large scans |
| Predictive machine learning | Off | Enable to catch novel/unknown threats |
| File concurrency | 8 | Files scanned in parallel per job |
| Max simultaneous scans | Unlimited | Cap concurrent scan jobs |
| Report mode | Malicious only | Set to "All files" to include clean files in the PDF |

### Scan Tags

Add custom labels (e.g. `laptop-audit`, `usb-check`) that appear in PDF reports and are forwarded to Vision One for filtering and correlation. Up to 32 tags.

---

## Scan History

Click **History** in the left menu to review all completed scans:
- Date and time started
- Scanned paths
- Files scanned / total files
- Malicious file count
- PDF download link
- Any errors

---

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `TM_V1_API_KEY` | Vision One API key (skip UI entry) | — |
| `TM_V1_REGION` | API region | — |
| `PORT` | HTTP port | `8080` |
| `V1FS_CONFIG_PATH` | Full path to config JSON file | OS-specific (see below) |
| `V1FS_REPORTS_DIR` | Directory for PDF reports | OS-specific (see below) |
| `V1FS_SCAN_CONCURRENCY` | Files scanned in parallel | `8` |

### Default data directories

| Platform | Location |
|----------|----------|
| Docker | `/data/` (persistent volume) |
| macOS | `~/Library/Application Support/V1FSScanner/` |
| Linux | `~/.config/V1FSScanner/` |
| Windows | `%APPDATA%\V1FSScanner\` |

---

## Supported Regions

| Region code | Location |
|-------------|----------|
| `us-east-1` | United States East |
| `eu-central-1` | Europe Central |
| `eu-west-2` | Europe West |
| `ca-central-1` | Canada |
| `ap-southeast-1` | Asia Pacific Southeast |
| `ap-southeast-2` | Asia Pacific Southeast 2 |
| `ap-northeast-1` | Asia Pacific Northeast |
| `ap-south-1` | Asia Pacific South |
| `me-central-1` | Middle East Central |

---

## Build from Source

Requires **Go 1.24+** and `make`.

```bash
git clone https://github.com/andrefernandes86/tool-v1fs-manual-scan.git
cd tool-v1fs-manual-scan

make docker          # Build Docker image
make darwin-arm64    # macOS Apple Silicon  → apps/macos/v1fs-scanner
make darwin          # macOS Intel          → apps/macos/v1fs-scanner
make darwin-app      # macOS .app bundle    → apps/macos/V1FSScanner.app
make linux           # Linux x86-64         → apps/linux/v1fs-scanner
make windows         # Windows x86-64       → apps/windows/v1fs-scanner.exe
make all             # Docker + all native binaries
make run             # Run locally for development (uses your local Go)
```

Web assets are embedded into the binary at build time — no separate `web/` folder is needed at runtime.

---

## Project Structure

```
apps/
  macos/
    run.sh              ← build + launch script (macOS)
    v1fs-scanner        ← compiled binary (git-ignored, built by run.sh)
    V1FSScanner.app/    ← .app bundle (git-ignored, built by make darwin-app)
  linux/
    run.sh              ← build + launch script (Linux)
    v1fs-scanner        ← compiled binary (git-ignored, built by run.sh)
  windows/
    v1fs-scanner.exe    ← compiled binary (git-ignored, built by make windows)
scripts/
  build-macos-app.sh    ← wraps binary in a .app bundle with Info.plist
web/                    ← UI assets (embedded into binary at build time)
internal/               ← Go source (API, scanner, config, store)
Makefile                ← build targets for all platforms
Dockerfile              ← containerised build and runtime
```

---

## Key Features

- **Four deployment options** — Docker, macOS, Linux, Windows
- **Self-contained binaries** — web UI embedded at build time, no external files needed at runtime
- **Web interface** — no command line required for day-to-day use
- **Real-time progress** — live file counters, throughput, ETA during scans
- **PDF reports** — malware details, file hashes, scan tags, optional clean-file list
- **Scan history** — review and re-download all past reports
- **Multiple scan paths** — queue up to 32 folders per job
- **Scan tags** — custom labels forwarded to Vision One
- **Custom malware actions** — log, quarantine, or delete on detection
- **Persistent config** — settings survive restarts on all platforms
- **Built-in malware test** — verify detection works before scanning real data (macOS/Linux)
- **Local scanner support** — connect to an on-premises gRPC File Security gateway

---

Powered by [Trend Vision One™ File Security SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk)
