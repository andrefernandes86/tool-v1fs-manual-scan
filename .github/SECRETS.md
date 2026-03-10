# Required secrets and variables for GitHub Actions

GitHub does not allow using `secrets` in workflow `if` conditions. So we use **repository variables** to turn steps on; you still store credentials in **Secrets**.

Add these under **Settings → Secrets and variables → Actions**:

---

## Repository variables (enable/disable steps)

| Variable | Value | Effect |
|----------|--------|--------|
| `PUBLISH_TO_DOCKERHUB` | `true` | Log in and push the image to Docker Hub. You must also set variable `DOCKERHUB_USERNAME` and secret `DOCKERHUB_TOKEN`. |
| `DOCKERHUB_USERNAME` | Your Docker Hub login (e.g. `johndoe`) | Used for login and for the `repo` job output so the report can show the image URL. Stored as a **variable** (not a secret) so GitHub doesn’t redact the output. |

**TMAS (Trend Vision One):** No variable needed. The TMAS scan runs on every workflow run; add the `TMAS_API_KEY` secret to authenticate. If the key is missing or invalid, the step fails and the report explains how to fix it.

**How to add variables:** Settings → Secrets and variables → Actions → **Variables** tab → **New repository variable**.

---

## Secrets (credentials – never in code)

### Docker Hub (for publish step)

| Name | Type | Description | Where to get it |
|------|------|-------------|------------------|
| `DOCKERHUB_USERNAME` | **Variable** | Your Docker Hub login (e.g. `johndoe`) | Your Docker Hub account. Use a variable so the `repo` job output is not redacted as a secret. |
| `DOCKERHUB_TOKEN`   | **Secret**   | Docker Hub access token (recommended) or password | [Docker Hub → Account Settings → Security → New Access Token](https://hub.docker.com/settings/security) |

**Note:** Prefer an **Access Token** with "Read, Write, Delete" for the repository over your account password.  
If you previously had `DOCKERHUB_USERNAME` as a secret, add it as a **variable** instead (same value) so the `repo` output is not skipped by GitHub.

### Trend Vision One – TMAS (for TMAS step)

| Secret name      | Description | Where to get it |
|------------------|-------------|------------------|
| `TMAS_API_KEY`   | Vision One API key with **"Run artifact scan"** permission | [Trend Vision One](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-artifact-scanner-tmas) → API Key |

The action uses `GITHUB_TOKEN` automatically for PR comments; no extra secret needed.

---

## Summary

- **To publish to Docker Hub:** Add variable `PUBLISH_TO_DOCKERHUB` = `true`, **variable** `DOCKERHUB_USERNAME`, and **secret** `DOCKERHUB_TOKEN`.
- **To run TMAS scan:** Add secret `TMAS_API_KEY` (Vision One API key with "Run artifact scan"). The scan runs every time; the report will show Trend Micro results or a clear error if the key is missing/invalid. Scans include: **vulnerabilities** and **secrets** on repo source (`dir:.`), **SBOM** on repo, **malware** on the built container image.
- **Build** always runs; no secrets or variables required.
