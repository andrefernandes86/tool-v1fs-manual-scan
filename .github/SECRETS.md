# Required secrets and variables for GitHub Actions

GitHub does not allow using `secrets` in workflow `if` conditions. So we use **repository variables** to turn steps on; you still store credentials in **Secrets**.

Add these under **Settings → Secrets and variables → Actions**:

---

## Repository variables (enable/disable steps)

| Variable | Value | Effect |
|----------|--------|--------|
| `TMAS_SCAN_ENABLED` | `true` | Run Trend Vision One (TMAS) scan. You must also set the `TMAS_API_KEY` secret. |
| `PUBLISH_TO_DOCKERHUB` | `true` | Log in and push the image to Docker Hub. You must also set `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets. |

**How to add:** Settings → Secrets and variables → Actions → **Variables** tab → **New repository variable**.  
If you don't set these, the corresponding steps are skipped and the pipeline report will say so.

---

## Secrets (credentials – never in code)

### Docker Hub (for publish step)

| Secret name           | Description | Where to get it |
|----------------------|-------------|------------------|
| `DOCKERHUB_USERNAME` | Your Docker Hub login (e.g. `johndoe`) | Your Docker Hub account |
| `DOCKERHUB_TOKEN`   | Docker Hub access token (recommended) or password | [Docker Hub → Account Settings → Security → New Access Token](https://hub.docker.com/settings/security) |

**Note:** Prefer an **Access Token** with "Read, Write, Delete" for the repository over your account password.

### Trend Vision One – TMAS (for TMAS step)

| Secret name      | Description | Where to get it |
|------------------|-------------|------------------|
| `TMAS_API_KEY`   | Vision One API key with **"Run artifact scan"** permission | [Trend Vision One](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-artifact-scanner-tmas) → API Key |

The action uses `GITHUB_TOKEN` automatically for PR comments; no extra secret needed.

---

## Summary

- **To publish to Docker Hub:** Add variable `PUBLISH_TO_DOCKERHUB` = `true` and secrets `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`.
- **To run TMAS scan:** Add variable `TMAS_SCAN_ENABLED` = `true` and secret `TMAS_API_KEY`.
- **Build** always runs; no secrets or variables required. The report step always runs and shows the status of each step.
