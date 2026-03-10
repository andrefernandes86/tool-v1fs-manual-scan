# Required secrets for GitHub Actions

Add these in **Settings → Secrets and variables → Actions** (repository secrets).  
The pipeline runs without them; steps that need a secret will show **Skipped** in the report until you add the value.

---

## Docker Hub (publish image)

| Secret name           | Description | Where to get it |
|----------------------|-------------|------------------|
| `DOCKERHUB_USERNAME` | Your Docker Hub login (e.g. `johndoe`) | Your Docker Hub account |
| `DOCKERHUB_TOKEN`   | Docker Hub access token (recommended) or password | [Docker Hub → Account Settings → Security → New Access Token](https://hub.docker.com/settings/security) |

**Note:** Prefer a **Access Token** with “Read, Write, Delete” for the repository over your account password.

---

## Trend Vision One – TMAS (optional scan)

| Secret name      | Description | Where to get it |
|------------------|-------------|------------------|
| `TMAS_API_KEY`   | Vision One API key with **“Run artifact scan”** permission | [Trend Vision One](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-artifact-scanner-tmas) → API Key |

The action uses `GITHUB_TOKEN` automatically for PR comments; no extra secret needed.  
If `TMAS_API_KEY` is not set, the **Trend Vision One (TMAS)** step is skipped and the report will say so. The pipeline does **not** fail when TMAS is skipped or when TMAS finds issues (non-blocking).

---

## Summary

- **Minimum to publish to Docker Hub:** set `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`.
- **To enable TMAS scan:** set `TMAS_API_KEY` and `TMAS_REGION`.
- No secrets are required for the **build** step; the report always runs and shows status for each step.
