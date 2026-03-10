# V1 File Security Scanner

Web application that uses the [Trend Vision One™ File Security Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk) to scan directories for malware. Runs as a Docker container.

## Features

- **Settings**: Configure and persist V1 API key and region (or use environment variables).
- **Folder browser**: Browse from root (`/`) and all OS directories; select a folder to scan.
- **Scan**: Start a scan that recurses through all files; live progress (current file and count) and a banner listing detected malicious files (name, path, malware name).
- **PDF report**: At the end of each scan a PDF report is generated, stored in the container, and available for download.
- **History**: Tab listing all scan tasks with statistics and a link to download the PDF.

## Environment variables

| Variable | Description |
|----------|-------------|
| `TM_V1_API_KEY` | Trend Vision One API key (optional if saved via UI). |
| `TM_V1_REGION` | Region (e.g. `us-east-1`; optional if saved via UI). |
| `PORT` | HTTP port (default `8080`). |
| `V1FS_CONFIG_PATH` | Path to save/load config JSON (default `/data/config.json`). |
| `V1FS_REPORTS_DIR` | Directory for PDF reports (default `/data/reports`). |

Secrets are not stored in source code; provide the API key via the console (env) or the web UI at first use.

## Build and run with Docker

```bash
# Build image (use --no-cache if you previously built with Go 1.21)
docker build --no-cache -t v1fs-scanner .

# Run (provide API key and region via env)
docker run -d -p 8080:8080 \
  -e TM_V1_API_KEY="your-api-key" \
  -e TM_V1_REGION="us-east-1" \
  -v v1fs-data:/data \
  --name v1fs-scanner \
  v1fs-scanner
```

Open http://localhost:8080. To scan a host directory, mount it and use the folder picker to select it, e.g.:

```bash
docker run -d -p 8080:8080 \
  -e TM_V1_API_KEY="your-api-key" \
  -e TM_V1_REGION="us-east-1" \
  -v v1fs-data:/data \
  -v /path/to/scan:/scan:ro \
  --name v1fs-scanner \
  v1fs-scanner
```

Then in the UI browse to `/scan` and select it to scan.

## Build locally (without Docker)

Requires Go 1.24+.

```bash
go mod tidy
go build -o v1fs-scanner .

# Run (optional: set TM_V1_API_KEY and TM_V1_REGION)
./v1fs-scanner
```

Static files are served from the `web/` directory (or current directory if `web/` is missing).

## Supported regions

- `us-east-1`, `eu-central-1`, `eu-west-2`, `ca-central-1`
- `ap-southeast-1`, `ap-southeast-2`, `ap-northeast-1`, `ap-south-1`, `me-central-1`
