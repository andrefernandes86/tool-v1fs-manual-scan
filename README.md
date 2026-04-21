# V1 File Security Scanner (V1FS Manual Scan Tool)

Docker-based web app that scans directories for malware using the [Trend Vision One™ File Security Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk). Configure API key and scan options in the UI, browse folders, run scans, and download PDF reports.

## Features

- **Settings**: Configure and persist scanner provider (TrendAI SaaS or on-prem **gRPC-only** local gateway), credentials/endpoint, optional TLS for gRPC, actions on detection (log only, move to quarantine, or delete), maximum simultaneous scans, predictive machine learning (SaaS only), optional file hash generation, report mode (summary vs all files), optional **Vision One scan tags** (type a tag and press **Enter** to add chips; each chip can be removed with **×**; up to **32** tags, sent with every file scan for filtering in Vision One; the tag **`v1fs-scanner`** is always added by the app), and **Test scanner connection** / **Compatibility check**.
- **Folder browser**: Platform-aware top level (**Locations**) — on Linux a single root (`/`); on macOS **System** (`/`) plus volumes under `/Volumes`; on Windows, available drive letters (`C:\`, `D:\`, …). Navigate with folder rows, **↑ Parent**, and **Locations**. Paths must be absolute (as shown in the UI).
- **Scan targets**: Build a list of one or more folders to scan in a single job (up to **32** paths). **Add current folder** appends the directory you are viewing; **Use only this folder** replaces the list with that path; **Clear all** empties the list. Scanning filesystem root (`/`) or a drive root asks for confirmation. **Cancel** on the report-name dialog does **not** start a scan.
- **Scan**: Live progress with **Target(s)** (the paths being scanned), elapsed time, files scanned, malicious count, and current file; banner listing detected malware; PDF report at the end. Each target directory is scanned **recursively** (every regular file under it). On Linux, only the **root** virtual trees **`/proc`**, **`/sys`**, **`/dev`**, and **`/run`** are skipped (not arbitrary folders elsewhere named `dev`, `sys`, or `run`). Permission errors on skipped system trees are treated as expected and do not inflate the scan error count. **Docker:** **`/`** inside the container is only the image (often a few hundred files). The UI shows a **Running in a container** notice with examples: mount the host (`-v /:/host:ro` → scan **`/host`** on Linux), your **macOS home** (`-v /Users/you:/mnt/data` → scan **`/mnt/data`**), or a USB/drive (`-v /mnt/usb:/mnt/usb` → scan **`/mnt/usb`**). See **FAQ** and **Quick start** below.
- **History**: List of past scans with statistics and download links for reports (multi-folder jobs show all paths separated by `; `).

---

## Screenshots

### Scanner tab — browse inside the container

Browse folders from the current path, tick **Scan targets** (each is scanned recursively), and use **Start scan** when ready. When the app runs in Docker, the **Running in a container** banner explains how to mount host folders or drives with `docker run -v` (for example `-v /Users/you:/mnt/data` then scan `/mnt/data`).

![Scanner tab with Docker hint, folder list under root, and scan targets](docs/screenshots/ui-scanner-docker-hint.png)

### Scanner tab — Locations (top level)

**Locations** shows platform-aware roots: on Linux in Docker you may see a single **Root** entry; on macOS you get **System** plus volumes; on Windows, drive letters. Open a row to descend; use **↑ Parent** to go up.

![Scanner tab with Locations and Root entry](docs/screenshots/ui-scanner-locations-root.png)

### Settings — scanner provider

Choose **TrendAI SaaS** or **on-prem gRPC gateway**, set endpoint and optional TLS, then **Test scanner connection**, **Compatibility check**, and **Save scanner settings**. With `-v v1fs-data:/data`, configuration persists across container restarts.

![Settings — scanner provider and on-prem gRPC gateway](docs/screenshots/ui-settings-scanner.png)

### Settings — actions & performance

Configure **log / quarantine / delete** on detection, hashes, PML (SaaS), concurrency, report mode, and optional **Vision One scan tags** as chips (**Enter** to add, **×** to remove). Click **Save actions** to persist.

![Settings — actions, performance, and scan tag chips](docs/screenshots/ui-settings-actions-performance.png)

### Settings — test options

Run **Submit EICAR test file** or **Submit clean test file** to copy built-in samples from `/data/test-samples` into a new `v1fs-test-…` folder under the destination you enter, then scan only that folder.

![Settings — test options for EICAR and clean samples](docs/screenshots/ui-settings-test-options.png)

### Scan in progress — detection example

Live progress shows targets, counts, and current file. When malware is found, paths appear under **Malicious files detected**; when the job finishes, use **Download PDF report**.

![Scan in progress with EICAR detection and scan details](docs/screenshots/ui-scan-progress-malicious.png)

---

## Quick start with Docker

```bash
# Build the image
docker build -t v1fs-scanner:latest .

# Run (replace with your API key and region, or set them in the UI after first start)
# -v v1fs-data:/data keeps config and reports across container recreates; add extra -v mounts for folders to scan.
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Add **extra `-v` mounts** for anything you want to scan: **host path first**, **container path second**, then add the **container** path under Scan targets.

Examples:

- **macOS (Docker Desktop):** `-v /Users/yourname:/mnt/data` then scan **`/mnt/data`** (replace `yourname`).
- **Linux USB:** `-v /mnt/usb:/mnt/usb` then scan **`/mnt/usb`**.
- **Linux full host (read-only):** `-v /:/host:ro` then scan **`/host`**.

Open **http://localhost:8080** in your browser. If you did not pass an API key via environment variables, go to **Settings** and enter your Trend Vision One API key and region, then click **Save scanner settings**.

After you change application code or pull updates, rebuild the image and **recreate** the container (same `docker stop` / `docker rm` / `docker run` pattern) so the running instance matches the new image; see **Keeping the container up to date** below.

---

## Scanning an external disk (USB or hard drive)

Mount the drive on the host, then pass it into the container with **`-v`**. Using the **same path** on host and container (e.g. `-v /mnt/usb:/mnt/usb`) keeps the UI aligned with what you see on the host. If you prefer different names, map them explicitly, e.g. **`-v /mnt/usb-drive:/mnt/external-drive`** and then scan **`/mnt/external-drive`** in the app.

### 1. Mount the drive on the host

**Linux**

```bash
sudo mkdir -p /mnt/usb
sudo mount /dev/sdb1 /mnt/usb
```

Use any name you like instead of `usb` (e.g. `external`, `backup`).

**macOS** — Volumes appear under `/Volumes/<VolumeName>`.  
**Windows** — Use a path Docker Desktop can access; share the drive in Docker settings if needed.

### 2. Run the container with the drive mounted

Use the **same path** inside the container as on the host to avoid confusion (e.g. `/mnt/usb` both sides).

- **Read-only (`:ro`)** — Use when you only want to **scan and report**. The app cannot quarantine or delete files. Use this if you do not want the solution to take any action on the drive.
- **Read-write (no `:ro`)** — Use when you want the app to **quarantine or delete** detected files on that drive.

**Example: scan only (no quarantine/delete)**

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/usb:ro \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

**Example: allow quarantine/delete on the drive**

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/usb \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

In the UI: **Scanner** → **Locations** → open **mnt** → **usb** → **Add current folder** (or **Use only this folder**) → **Start scan**. Confirm the optional report name in the dialog (**Cancel** aborts and does not start the scan).

If the container is already running, stop and remove it first: `docker stop v1fs-scanner && docker rm v1fs-scanner`, then run the `docker run` command again.

---

## How to use the web interface

### 1. Configure scanner connection

1. Open **http://localhost:8080** (or your host:port).
2. In the left navigation, choose **Settings**.
3. Under **V1 File Security settings**, choose **Scanner provider**:
   - **TrendAI SaaS scanner**:
     - Enter your **Trend Vision One API key** (with “Run file scan via SDK” permission).
     - Choose the **Region** that matches your API key (e.g. `us-east-1`).
   - **On-prem gRPC gateway** (local scanner):
     - The app talks to the Vision One File Security gateway **only over gRPC** (not HTTP/REST).
     - Enter **host:port**, e.g. `192.168.200.71:31050` (Kubernetes NodePort) or `192.168.200.71:50051` if you connect directly to the gateway port.
     - Enable **Use TLS for gRPC** when the gateway expects an encrypted channel (or use `grpcs://` / `https://` in the endpoint field to force TLS).
     - Optionally enter a local scanner API key/token.
4. Click **Test scanner connection** to verify reachability (gRPC client setup). Use **Compatibility check** to run the same **ScanFile → ScanBuffer** probe path as directory scans. If the check reports a version-handshake warning, run an **EICAR test scan** under **Settings → Test options** to confirm end-to-end behavior; if real scans keep failing, align gateway and [File Security Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk) versions.
5. Click **Save scanner settings**.  
   You can also set `TM_V1_API_KEY` and `TM_V1_REGION` at startup; these are used for SaaS mode when saved values are empty.

### Kubernetes + V1FS Service Gateway note

If your V1FS service is exposed by Kubernetes `NodePort`, use the node IP and NodePort of the scanner gRPC service.

Example from your service list:

- Service: `scanner-nodeport`
- gRPC port mapping: `50051:31050/TCP`
- Use in app: `<k8s-node-ip>:31050` (for example `192.168.200.71:31050`)

Port `50051` is the in-cluster scanner port; `31050` is the external NodePort to use from outside the cluster.

### 2. Configure actions on detection

In **Settings**, under **Actions on detection**:

- **Log only** — Only record findings in the report and banner; files are not moved or deleted.
- **Move to quarantine folder** — Move detected files to a folder you choose. When selected, enter the **Quarantine folder path** (e.g. `/data/quarantine`). Use the Scanner tab to browse and copy a path if needed.
- **Delete** — Permanently delete detected malicious files.
- **Generate file hashes for malicious files** — When enabled, the scanner computes a SHA-256 hash for each malicious file and includes it in the PDF report. This adds extra I/O and CPU work, so disabling it keeps scans faster.
- **Enable predictive machine learning (PML)** — When enabled for the **SaaS** scanner, the app sends PML/feedback hints as described in the vendor documentation. **Local gRPC** scans do not send these hints (gateway compatibility).
- **Maximum simultaneous scans** — How many scan jobs can run at the same time. Set `0` for unlimited (up to a maximum of 1000). Lower values can protect system resources and API quota when multiple scans are triggered.
- **Report generation mode** — **Statistics only** (default) keeps the PDF smaller; **All files** lists every clean file as well as malicious ones (can be very large for big scans). Save under **Actions & performance** with **Save actions**.
- **Vision One scan tags (optional)** — Add labels that Vision One can use to filter activity. Type one tag in the field, press **Enter** to add it as a chip; click **×** on a chip to remove it. Up to **32** tags, 128 characters each; duplicates and control characters are rejected. The app always adds **`v1fs-scanner`** on the server side in addition to your tags (and any PML-related tags in SaaS mode).

Click **Save actions** after changing.

### 3. Run a scan task

1. In the left navigation, choose **Scanner**.
2. **Browse**: Click **Locations** to see top-level roots (depends on OS — see **Features**). Open folders by clicking rows; use **↑ Parent** to go up. Clicking **Locations** only changes the browser — it does not clear your scan target list (use **Clear all** for that).
3. **Scan targets** (right side):
   - **Add current folder** — Adds the folder shown in the breadcrumb trail (e.g. ` → /mnt/usb`) to the list. Duplicates are ignored. Maximum **32** folders per scan.
   - **Use only this folder** — Clears the list and sets a single target to the folder you are viewing.
   - **Clear all** — Removes every target. **Start scan** stays disabled until at least one folder is listed.
   - Including **`/`** (Linux root) or a **Windows drive root** (`C:\`, etc.) triggers a confirmation dialog because the scope is very large.
   - **Docker:** **`/`** is the container root only (often far fewer files than the host). Recreate `docker run` with **`-v host:container`**, then add the **container** path here: **macOS home** — `-v /Users/you:/mnt/data` → **`/mnt/data`**; **Linux host** — `-v /:/host:ro` → **`/host`**; **USB** — `-v /mnt/usb:/mnt/usb` → **`/mnt/usb`** (or map to another name, e.g. `/mnt/external-drive`). The Scanner tab also shows a **Running in a container** help banner when applicable.
4. Click **Start scan**. A dialog asks for an optional **report name** (for the PDF and **History**). **OK** starts the job; **Cancel** closes the dialog and **does not** start a scan.
5. Under **Scan in progress**, **Target(s)** shows the path or paths being scanned (semicolon-separated if you chose several). You also see **Elapsed**, **Files scanned**, **Malicious found**, **Scan errors**, and the file currently being scanned. When finished, use **Download PDF report** and review the **Malicious files detected** banner if anything was found (an example is shown under **Screenshots** → **Scan in progress — detection example**).

**Multiple folders in one job** — Add several paths (e.g. `/data/project-a` and `/data/project-b`) before **Start scan**. The backend merges file lists (no duplicate paths), produces one PDF, and one history entry with all targets in the path field.

**API note** — `POST /api/scan/start` accepts either `paths` (array of absolute directory strings, up to 32) or legacy `path` (single string).

### 4. Check results

- **Scanner** — Progress, malicious count, and list of detected files; **Download PDF report** when the scan finishes.
- **History** — Past scans with **report name**, path, time, counts, and a **Download PDF** link per report.
- **Settings → Test options** — Use **Submit EICAR test file** or **Submit clean test file** to copy a built‑in sample (`/data/test-samples`: EICAR + clean file) into a **new subfolder** under the destination you choose (`v1fs-test-<timestamp>`), then scan **only that subfolder**. That way a destination like `/data` does not recursively include `test-samples/eicar.com` during a clean test. The optional report-name dialog works the same way: **Cancel** does not start the test scan. The app switches to the **Scanner** tab and shows progress for that test run (independent of the **Scan targets** list).

---

## Environment variables

| Variable | Description |
|----------|-------------|
| `TM_V1_API_KEY` | Trend Vision One API key (optional if set in the UI). |
| `TM_V1_REGION` | Region (e.g. `us-east-1`; optional if set in the UI). |
| `PORT` | HTTP port inside the container (default `8080`). |
| `V1FS_CONFIG_PATH` | Path to config file (default `/data/config.json`). |
| `V1FS_REPORTS_DIR` | Directory for PDF reports (default `/data/reports`). |
| `V1FS_TEST_SAMPLES_DIR` | Directory for test samples (default `/data/test-samples`). |
| `V1FS_SCAN_CONCURRENCY` | Number of files scanned in parallel (default `8`). This is an advanced server-side tuning option; in most cases you can leave it at the default. |

Secrets are not stored in the image; use environment variables or the web UI.

---

## Keeping the container up to date

To run the latest application and dependencies (Go, SDK, OS base), update the image and recreate the container.

### Option A: Pull and run from a registry (if you publish the image)

If you push the image to Docker Hub or GitHub Container Registry:

```bash
# Pull latest image
docker pull <your-registry>/v1fs-scanner:latest

# Stop and remove the current container (keeps your volume)
docker stop v1fs-scanner
docker rm v1fs-scanner

# Run the new image (same volumes as before)
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/usb \
  --name v1fs-scanner \
  <your-registry>/v1fs-scanner:latest
```

Adjust the second `-v` to match how you mount host folders (e.g. `-v /Users/you:/mnt/data` on macOS). Your config and reports stay in the **`v1fs-data`** volume mapped to **`/data`**.

### Option B: Rebuild from source (this repository)

When you pull the latest code from this repo, rebuild the image and recreate the container:

```bash
# Get latest code
git pull origin main

# Rebuild image (use --no-cache to refresh all layers and dependencies)
docker build --no-cache -t v1fs-scanner:latest .

# Stop and remove the old container
docker stop v1fs-scanner
docker rm v1fs-scanner

# Run the new image (same volume and drive mount)
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/usb \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

- **`--no-cache`** forces a full rebuild so base image, Go version, and dependencies (including the Trend Micro SDK) are updated.
- Use the same `-v v1fs-data:/data` (and same drive mount if you use one) so config and reports stay available.

### What gets updated

- Application code (Go app, web UI).
- Base images (e.g. `golang:1.24.4-alpine`, `alpine:3.19`).
- Go modules (including [tm-v1-fs-golang-sdk](https://github.com/trendmicro/tm-v1-fs-golang-sdk)) when you rebuild with `go mod tidy` in the Docker build.

### What is preserved

- Config (API key, region, actions, quarantine path, scan concurrency, scan tags, report mode, and related settings) and PDF reports in the **Docker volume** (`v1fs-data`). They persist as long as you keep the volume and pass it to the new container with `-v v1fs-data:/data`.

---

## Build and run (reference)

### Docker

```bash
docker build -t v1fs-scanner:latest .
docker run -d -p 8080:8080 -v v1fs-data:/data --name v1fs-scanner v1fs-scanner:latest
```

### Local (without Docker)

Requires **Go 1.24+**.

```bash
go mod tidy
go build -o v1fs-scanner .
./v1fs-scanner
```

Static files are served from the `web/` directory (or current directory if `web/` is missing).

---

## FAQ: “I scanned `/` but only ~300 files”

The scan **is** recursive. That file count usually means the app is running **inside Docker**: `/` is the **container’s** root (Alpine + app), not your laptop or server’s disk, and not an external drive unless you mounted it.

**Scan the whole host** — bind-mount the host and scan that path:

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /:/host:ro \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Then in the UI open **`/host`** (e.g. **Locations** → **Root** → **host**) and add it as a scan target.

**Scan your Mac home folder from Docker Desktop** — mount it under a path you will select in the app (config still uses the **`/data`** volume):

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /Users/andre:/mnt/data:ro \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Replace **`/Users/andre`** with your username. In the UI, browse to **`/mnt/data`** and add it under **Scan targets**.

**Scan an external USB or hard drive** — mount the drive on the host first, then pass it into the container with **`-v`**. Same path on both sides is often easiest:

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/usb:ro \
  --name v1fs-scanner \
  v1fs-scanner:latest
```

Or map to another in-container name, e.g. **`-v /mnt/usb-drive:/mnt/external-drive:ro`**, then scan **`/mnt/external-drive`**. Use **`:ro`** if you only want scanning and reporting (no quarantine/delete on that volume).

The UI also shows a **Running in a container** notice when it detects Docker.

---

## Supported regions

- `us-east-1`, `eu-central-1`, `eu-west-2`, `ca-central-1`
- `ap-southeast-1`, `ap-southeast-2`, `ap-northeast-1`, `ap-south-1`, `me-central-1`

Use the region that matches your Trend Vision One API key.
