# Run the comic scraper ON THE HOST (SPEC-02 P1.8) — needed because the containerized
# Chrome is blocked by Cloudflare, while your host's real Chrome passes.
#
# Prereqs (one-time):
#   pip install -r requirements.txt          # fastapi uvicorn boto3 seleniumbase ...
#   In docker-compose.override.yml, expose api + minio to the host:
#     services: { minio: { ports: ["9000:9000"] }, api: { ports: ["8080:8080"] } }
#   In .env set   COMIC_SCRAPER_URL=http://host.docker.internal:8000
#   then          docker compose up -d api minio
#
# Then just run:  powershell -ExecutionPolicy Bypass -File scraper\run-host.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$secret = (Get-Content "$root\.env" | Select-String "^COMIC_SYNC_SECRET=").ToString().Split("=", 2)[1].Trim()

$env:COMIC_SYNC_SECRET       = $secret
$env:S3_ENDPOINT             = "http://localhost:9000"                                   # MinIO exposed to host
$env:S3_ACCESS_KEY           = "portal"
$env:S3_SECRET_KEY           = "change-me"
$env:S3_BUCKET               = "portal-media"
$env:S3_REGION               = "us-east-1"
$env:COMIC_SYNC_CALLBACK_URL = "http://localhost:8080/api/v1/internal/comic/sync-callback"  # api exposed to host
$env:SCRAPER_HEADLESS        = "true"                                                    # real Chrome, no popup window
$env:SCRAPER_DOWNLOAD_WORKERS = "4"
$env:PYTHONUTF8              = "1"                                                       # emit UTF-8 so Vietnamese log lines aren't mojibake
$env:PYTHONIOENCODING        = "utf-8"

# Resolve python's full path — under Task Scheduler `python` may be absent from PATH
# or shadowed by the Windows Store alias.
$py = (Get-Command python.exe -ErrorAction SilentlyContinue | Where-Object { $_.Source -notlike "*WindowsApps*" } | Select-Object -First 1).Source
if (-not $py) {
  $py = @(
    "$env:LOCALAPPDATA\Programs\Python\Python313\python.exe",
    "$env:LOCALAPPDATA\Programs\Python\Python312\python.exe"
  ) | Where-Object { Test-Path $_ } | Select-Object -First 1
}
if (-not $py) { throw "python.exe not found — set `$py` in run-host.ps1" }

$logf = Join-Path $PSScriptRoot "scraper-host.log"
Set-Location $PSScriptRoot
Add-Content -Path $logf -Value "$(Get-Date -Format o)  starting [python: $py]" -Encoding UTF8
# uvicorn logs to STDERR — with ErrorActionPreference=Stop, PowerShell would treat
# those normal log lines as a fatal error and kill the server. Switch to Continue so
# the (blocking) server runs; *>&1 merges every stream into the pipeline and
# Add-Content writes each line as UTF-8 (plain-text readable — *>> defaulted to UTF-16).
$ErrorActionPreference = "Continue"
# -u = unbuffered, so scrape progress lines appear in the log immediately.
& $py -u -m uvicorn app:app --host 127.0.0.1 --port 8000 *>&1 |
  ForEach-Object { Add-Content -Path $logf -Value $_ -Encoding UTF8 }
