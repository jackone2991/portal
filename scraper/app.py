"""FastAPI wrapper for the comic scraper (SPEC-02 P1.8).

The Go comic module POSTs /sync; we accept it, run the scrape in a single-worker
thread pool (one headful Chrome at a time — concurrent browsers would OOM), and
return 202 immediately. Completion is reported to Portal via the sync-callback
inside run_sync.
"""

import concurrent.futures

from fastapi import FastAPI
from pydantic import BaseModel

from scraper import run_sync, cancel_sync

app = FastAPI(title="portal-comic-scraper")

# Serialize *syncs*: one comic at a time, extra requests queue here. The parallelism
# lives inside run_sync, which fans one comic's chapters out over SCRAPER_SYNC_WORKERS
# browsers — so a second comic waits rather than competing for RAM with the first.
_pool = concurrent.futures.ThreadPoolExecutor(max_workers=1)


class SyncReq(BaseModel):
    source_id: str
    source_url: str
    chapters: str = ""
    existing: str = ""  # comma list of chapter labels already imported (skip on full sync)


class CancelReq(BaseModel):
    source_id: str


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/sync", status_code=202)
def sync(req: SyncReq):
    _pool.submit(run_sync, req.source_id, req.source_url, req.chapters, req.existing)
    return {"accepted": True, "source_id": req.source_id}


@app.post("/cancel")
def cancel(req: CancelReq):
    cancel_sync(req.source_id)  # run_sync stops before its next chapter
    return {"cancelled": True, "source_id": req.source_id}
