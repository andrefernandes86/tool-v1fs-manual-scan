# V1 File Security Scanner (V1FS Manual Scan Tool)

Docker-based web app that scans directories for malware using the [Trend Vision One™ File Security Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk). Configure API key and scan options in the UI, browse folders, run scans, and download PDF reports.

## Features

- **Settings**: Configure and persist V1 API key, region, and actions on detection (log only, move to quarantine, or delete).
- **Folder browser**: Browse from root (`/`) and all directories; select a folder to scan (including mounted external drives).
- **Scan**: Live progress (elapsed time, files scanned, malicious count, current file path); banner listing detected malware; PDF report at the end.
- **History**: List of past scans with statistics and download links for reports.

---

## Quick start with Docker

```bash
# Build the image
docker build -t v1fs-scanner .

# Run (replace with your API key and region, or set them in the UI after first start)
docker run -d -p 8080:8080 \
  -v /mnt/data:/mnt/data \ # Update the /mnt/data with the path you would like to scan
  --name v1fs-scanner \
  v1fs-scanner
```

Open **http://localhost:8080** in your browser. If you did not pass an API key via environment variables, go to **Settings** and enter your Trend Vision One API key and region, then click **Save API key & region**.

---

## Scanning an external disk (USB or hard drive)

Mount the drive on the host, then pass it into the container. Use the **same path** on both sides (e.g. `/mnt/usb` on host and in container) so the UI shows a path you recognise.

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
  -v /mnt/usb:/mnt/usb:ro \
  --name v1fs-scanner \
  v1fs-scanner
```

**Example: allow quarantine/delete on the drive**

```bash
docker run -d -p 8080:8080 \
  -v /mnt/usb:/mnt/usb \
  --name v1fs-scanner \
  v1fs-scanner
```

In the UI: **Scanner** → browse **/** → **mnt** → **usb** → **Use this folder** → **Start scan**.

If the container is already running, stop and remove it first: `docker stop v1fs-scanner && docker rm v1fs-scanner`, then run the `docker run` command again.

---

## How to use the web interface

### 1. Configure API key and region

1. Open **http://localhost:8080** (or your host:port).
2. Go to the **Settings** tab.
3. Under **V1 File Security settings**:
   - Enter your **Trend Vision One API key** (with “Run file scan via SDK” permission).
   - Choose the **Region** that matches your API key (e.g. `us-east-1`).
4. Click **Save API key & region**.  
   You can also set `TM_V1_API_KEY` and `TM_V1_REGION` when starting the container; the UI then uses those until you save new values.

### 2. Configure actions on detection

In **Settings**, under **Actions on detection**:

- **Log only** — Only record findings in the report and banner; files are not moved or deleted.
- **Move to quarantine folder** — Move detected files to a folder you choose. When selected, enter the **Quarantine folder path** (e.g. `/data/quarantine`). Use the Scanner tab to browse and copy a path if needed.
- **Delete** — Permanently delete detected malicious files.

Click **Save actions** after changing.

### 3. Run a scan task

1. Open the **Scanner** tab.
2. **Folder to scan**: Click **Root**, then open the folder you want (e.g. **mnt** → **usb** for a drive mounted at `/mnt/usb`). Click **Use this folder**, then **Start scan**.
3. In **Scan in progress** you see: **Elapsed** time, **Files scanned** (current/total), **Malicious found**, and the **Scanning** path. When done, use **Download PDF report** and check the **Malicious files detected** banner.

### 4. Check results

- **Scanner tab** — Progress, malicious count, and list of detected files; **Download PDF report** when the scan finishes.
- **History tab** — Past scans with path, time, counts, and a **Download PDF** link per report.
- **Settings → Test options** — **Run test scan** to scan built-in samples (`/data/test-samples`: EICAR + clean file) and verify detection.

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

# Run the new image (same volume and drive mount)
docker run -d -p 8080:8080 \
  -v /mnt/usb:/mnt/usb \
  --name v1fs-scanner \
  <your-registry>/v1fs-scanner:latest
```

Your config and reports in the `v1fs-data` volume are preserved.

### Option B: Rebuild from source (this repository)

When you pull the latest code from this repo, rebuild the image and recreate the container:

```bash
# Get latest code
git pull origin main

# Rebuild image (use --no-cache to refresh all layers and dependencies)
docker build --no-cache -t v1fs-scanner .

# Stop and remove the old container
docker stop v1fs-scanner
docker rm v1fs-scanner

# Run the new image (same volume and drive mount)
docker run -d -p 8080:8080 \
  -v /mnt/usb:/mnt/usb \
  --name v1fs-scanner \
  v1fs-scanner
```

- **`--no-cache`** forces a full rebuild so base image, Go version, and dependencies (including the Trend Micro SDK) are updated.
- Use the same `-v v1fs-data:/data` (and same drive mount if you use one) so config and reports stay available.

### What gets updated

- Application code (Go app, web UI).
- Base images (e.g. `golang:1.24.4-alpine`, `alpine:3.19`).
- Go modules (including [tm-v1-fs-golang-sdk](https://github.com/trendmicro/tm-v1-fs-golang-sdk)) when you rebuild with `go mod tidy` in the Docker build.

### What is preserved

- Config (API key, region, actions, quarantine path) and PDF reports in the **Docker volume** (`v1fs-data`). They persist as long as you keep the volume and pass it to the new container with `-v v1fs-data:/data`.

---

## Build and run (reference)

### Docker

```bash
docker build -t v1fs-scanner .
docker run -d -p 8080:8080 -v v1fs-data:/data --name v1fs-scanner v1fs-scanner
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

## Supported regions

- `us-east-1`, `eu-central-1`, `eu-west-2`, `ca-central-1`
- `ap-southeast-1`, `ap-southeast-2`, `ap-northeast-1`, `ap-south-1`, `me-central-1`

Use the region that matches your Trend Vision One API key.
