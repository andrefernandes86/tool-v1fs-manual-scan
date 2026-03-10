# V1 File Security Scanner (tool-v1fs-manual-scan)

Web application that uses the [Trend Vision One™ File Security Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk) to scan directories for malware. Runs as a Docker container. Configure API key and scan options in the UI, browse folders, run scans, and download PDF reports.

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
  -v v1fs-data:/data \
  --name v1fs-scanner \
  v1fs-scanner
```

Open **http://localhost:8080** in your browser. If you did not pass an API key via environment variables, go to **Settings** and enter your Trend Vision One API key and region, then click **Save API key & region**.

---

## Scanning an external disk (USB drive or hard drive)

To scan a USB stick or external hard drive connected to the host, mount it under **`/mnt/<drive>`** on the host and pass that path into the container. The **recommended scan target** in the web UI is **`/mnt/drive`** (see below).

### 1. Mount the drive on the host (recommended: `/mnt/<drive>`)

**Linux**

- List block devices: `lsblk` or `fdisk -l`
- Create a mount point and mount the partition, e.g.:
  ```bash
  sudo mkdir -p /mnt/usb
  sudo mount /dev/sdb1 /mnt/usb
  ```
  Use any name instead of `usb` (e.g. `external`, `backup`). We recommend **`/mnt/<drive>`** so you can use the same Docker command pattern below.

**macOS**

- The volume usually appears under `/Volumes/<VolumeName>`. You can symlink or bind it under `/mnt` if you want the same paths, or use `/Volumes/<VolumeName>` directly in the Docker `-v` below.

**Windows (Docker Desktop)**

- Use a path Docker Desktop can access; you may need to share the drive in Docker Desktop settings.

### 2. Run the container with the external drive mounted at `/mnt/drive`

Mount your host path into the container at **`/mnt/drive`**. That way the **recommended target to scan** in the UI is always **`/mnt/drive`**.

**Example: drive mounted on host at `/mnt/usb`**

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/drive:ro \
  --name v1fs-scanner \
  v1fs-scanner
```

- Replace `/mnt/usb` with your host path (e.g. `/mnt/external`, `/mnt/backup`, or `/Volumes/MyUSB` on macOS).
- The container sees the drive at **`/mnt/drive`**.
- In the web UI: **Scanner** tab → browse to **/** → **mnt** → **drive** → **Use this folder** → **Start scan**. So the **recommended scan target** is **`/mnt/drive`**.

**Example: drive at `/mnt/external` on the host**

```bash
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/external:/mnt/drive:ro \
  --name v1fs-scanner \
  v1fs-scanner
```

Again, in the UI browse to **/** → **mnt** → **drive** and use that as the scan target.

**Important**

- Use the actual mount path of the drive on your host; replace `/mnt/usb` or `/mnt/external` with your path.
- `:ro` is optional but recommended so the container cannot modify the external drive.
- If the container is already running, stop and remove it, then run the command again with the new volume:  
  `docker stop v1fs-scanner && docker rm v1fs-scanner` then `docker run ...` as above.

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
2. **Folder to scan**:
   - Use **Root** to start from `/`.
   - Click a folder name to open it. For an external drive mounted at **`/mnt/drive`**, browse **/** → **mnt** → **drive**, then click **Use this folder** (the path appears under “Selected:”).
   - The **recommended scan target** for an external disk is **`/mnt/drive`** when you start the container with `-v /mnt/<your-drive>:/mnt/drive:ro`.
3. Click **Start scan**.
4. Watch **Scan in progress**:
   - **Elapsed** — Time since the scan started.
   - **Files scanned** — Count of files scanned so far and total.
   - **Malicious found** — Number of files detected as malicious so far.
   - **Scanning** — Full path of the file currently being scanned.
5. When the scan finishes, you can **Download PDF report** from the same card. Detected files are also listed in the **Malicious files detected** banner.

### 4. Check results

- **During/after scan (Scanner tab)**  
  Progress, malicious count, and “Malicious files detected” list with file name, path, and malware name. Use **Download PDF report** when the scan is done.

- **History tab**  
  List of past scans: path scanned, start time, files scanned, malicious count, and a **Download PDF** link for each report.

- **Test scan**  
  In **Settings** → **Test options**, use **Run test scan** to scan the built-in test samples (`/data/test-samples`: EICAR and a clean file) and confirm detection works.

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

# Run the new image with the same volume and external drive mount
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/drive:ro \
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

# Run the new image (reuse your volume and external drive at /mnt/drive)
docker run -d -p 8080:8080 \
  -v v1fs-data:/data \
  -v /mnt/usb:/mnt/drive:ro \
  --name v1fs-scanner \
  v1fs-scanner
```

- **`--no-cache`** forces a full rebuild so base image, Go version, and dependencies (including the Trend Micro SDK) are updated.
- Use the same `-v v1fs-data:/data` (and same external `-v` if you use one) so config and reports stay available.

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
