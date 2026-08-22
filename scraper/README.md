# Comic scraper service (SPEC-02 P1.8)

A small FastAPI service that scrapes an external comic source and hands the result
to Portal's existing comic zip-import pipeline. It exists as a **separate Python
service** because the hard part — bypassing Cloudflare Turnstile — needs a real
Chrome driven by `undetected-chromedriver` (SeleniumBase), which the Go backend
can't do reliably.

## Flow

```
Go comic module (POST /sync) → this service:
  discover chapters (one browser) → fan out over SCRAPER_SYNC_WORKERS browsers, each:
    scrape image URLs (uc Chrome) → download images → folder-per-chapter zip
    → hand to the upload pool: MinIO import/{import_id}.zip → POST sync-callback
→ Go enqueues comic:import_zip → the optimized import worker creates chapters+pages
```

Chapters therefore arrive **out of order**. That is safe because the importer derives
a chapter's `sort_order` from the number in its title rather than from arrival order
(`chapterSortOrder` in `backend/internal/modules/comic/import.go`) — if you ever change
how chapters are named, keep that parser in step or the reader order will scramble.

The zip layout (`{chapter}/{001}.jpg`) is exactly what the multi-chapter import
consumes, so the whole optimized pipeline (parallel ingest, progressive paging) is
reused untouched.

## Endpoints

- `GET /health`
- `POST /sync {source_id, source_url, chapters, existing}` → `202` (runs async, one
  *comic* at a time — the parallelism is per-chapter inside a sync). `chapters` is
  optional: blank = all discovered; `"A-B"` = a numeric chapter range; or explicit
  chapter URLs. `existing` is a comma list of chapter labels to skip.
- `POST /cancel {source_id}` → stops the sync before its next chapter.

## Config (env)

`S3_ENDPOINT` `S3_ACCESS_KEY` `S3_SECRET_KEY` `S3_BUCKET` `S3_REGION`
`COMIC_SYNC_CALLBACK_URL` (default `http://api:8080/api/v1/internal/comic/sync-callback`)
`COMIC_SYNC_SECRET` (must match the API's) · `SCRAPER_DOWNLOAD_WORKERS` (default 4).

Throughput knobs (see `.env.example` for the trade-offs): `SCRAPER_SYNC_WORKERS`
(browsers per comic, default 3) · `SCRAPER_RECONNECT_TIME` (uc detach seconds,
default 3 — raise if Cloudflare starts challenging) · `SCRAPER_LAZY_TIMEOUT` ·
`SCRAPER_BATCH_SIZE` (chapters per zip/import round-trip).

## ⚠️ Cloudflare in Docker — run on the host instead

Undetected Chrome in a **container** (even headful on xvfb) is fingerprinted and
**blocked by Cloudflare** on some sites — e.g. truyenqq returns the "Just a moment…"
challenge, so the container scrapes 0 images. The container shares the host's public
IP, so this is a *browser-fingerprint* problem, not an IP one: your **host's real
Chrome passes** (your manual script proves it). So run this service on the host.

### Turnkey host setup (Windows dev) — verified working

Python `getaddrinfo` doesn't resolve `*.portal.localhost`, so the host scraper
reaches the api via its **exposed port**, not the Traefik hostname.

1. **Expose api + MinIO to the host** — add to `docker-compose.override.yml`:
   ```yaml
   services:
     minio: { ports: ["9000:9000"] }
     api:   { ports: ["8080:8080"] }
   ```
2. **Point the API at the host scraper** — in `.env`:
   `COMIC_SCRAPER_URL=http://host.docker.internal:8000`
3. Apply: `docker compose up -d api minio`
4. **Install deps + run the scraper on the host** (real Chrome → bypasses Cloudflare):
   ```powershell
   cd scraper
   pip install -r requirements.txt        # one-time
   powershell -ExecutionPolicy Bypass -File run-host.ps1
   ```
   `run-host.ps1` sets the env (`S3_ENDPOINT=http://localhost:9000`,
   `COMIC_SYNC_CALLBACK_URL=http://localhost:8080/...`, `SCRAPER_HEADLESS=true`) and
   starts uvicorn. To keep it running after login, register it as a Windows Task /
   `nssm` service.

Now the dockerized API triggers your host scraper, which scrapes with real Chrome,
uploads the zip to MinIO, and calls back — the rest of the pipeline is unchanged.
**Verified:** truyenqqko → 2 chapters / 28 pages imported end-to-end.

The scraping selectors (`.chapter_content .page-chapter img`, `a[href*="-chap-"]`)
target truyenqq-style sites; adjust `scraper.py` for other sources.
