"""Comic scraper worker (SPEC-02 P1.8).

Adapted from the user's SeleniumBase scraper. Given a comic source URL it discovers
the chapter list, scrapes each chapter's image URLs (Cloudflare-bypass via
undetected-chromedriver + xvfb), downloads images into a folder-per-chapter tree,
zips it, uploads the zip to MinIO/S3 at the import job's key, and calls the Portal
sync-callback. The folder-per-chapter zip is exactly what the Go comic:import_zip
worker consumes, so all of Portal's optimized import pipeline is reused.
"""

import concurrent.futures
import io
import ipaddress
import os
import queue
import re
import shutil
import socket
import tempfile
import threading
import time
import zipfile
from urllib.parse import urljoin, urlparse

import boto3
import requests
from botocore.client import Config
from seleniumbase import Driver

# ── config (env) ────────────────────────────────────────────────────────
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "http://minio:9000")
S3_ACCESS_KEY = os.getenv("S3_ACCESS_KEY", "portal")
S3_SECRET_KEY = os.getenv("S3_SECRET_KEY", "change-me")
S3_BUCKET = os.getenv("S3_BUCKET", "portal-media")
CALLBACK_URL = os.getenv("COMIC_SYNC_CALLBACK_URL", "http://api:8080/api/v1/internal/comic/sync-callback")
INTERNAL_SECRET = os.getenv("COMIC_SYNC_SECRET", "")
DOWNLOAD_WORKERS = int(os.getenv("SCRAPER_DOWNLOAD_WORKERS", "4"))
# Chapters per import batch. Default 1 = import each chapter the moment it's scraped
# (finest granularity; nothing already-scraped is lost if a later chapter is blocked).
# Raise it for a fast, non-Cloudflare source to cut per-chapter import overhead.
BATCH_SIZE = int(os.getenv("SCRAPER_BATCH_SIZE", "1"))
BATCH_RETRIES = int(os.getenv("SCRAPER_BATCH_RETRIES", "3"))  # retry a fully-failed batch
# uc-chromedriver stays disconnected this long after a navigation so Cloudflare can't
# see the automation hooks. It is a hard per-chapter cost, so it is tunable: lower is
# faster, too low gets you challenged. 3s held up on truyenqq; raise it if "Just a
# moment..." starts showing up in the log.
RECONNECT_TIME = float(os.getenv("SCRAPER_RECONNECT_TIME", "3"))
# Chapters of one comic are scraped by this many browsers at once. Each is a real
# Chrome (~1GB), and each multiplies the request rate this host aims at the source —
# so this is the knob to back off first if the site starts challenging.
SYNC_WORKERS = max(1, int(os.getenv("SCRAPER_SYNC_WORKERS", "3")))
# Ceiling for the adaptive lazy-load poll (it normally exits long before this).
LAZY_TIMEOUT = float(os.getenv("SCRAPER_LAZY_TIMEOUT", "12"))

DL_HEADERS = {
    "Accept": "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
    "(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
}


def _s3():
    return boto3.client(
        "s3",
        endpoint_url=S3_ENDPOINT,
        aws_access_key_id=S3_ACCESS_KEY,
        aws_secret_access_key=S3_SECRET_KEY,
        config=Config(signature_version="s3v4", s3={"addressing_style": "path"}),
        region_name=os.getenv("S3_REGION", "us-east-1"),
    )


def _referer(url: str) -> str:
    m = re.match(r"^(https?://[^/]+)", url)
    return (m.group(1) + "/") if m else url


# ── SSRF guard ────────────────────────────────────────────────────────────────
# Mirrors backend/internal/modules/comic/sourceguard.go. The Go side validates a
# source when it is created and again when a sync is triggered; this is the last
# line of defence, re-resolving at fetch time so DNS rebinding between those two
# points loses, and covering URLs discovered from the source page (chapter links,
# image srcs) which the Go side never sees. This process sits on the compose
# `internal` network, so an unchecked fetch reaches api:8080, minio:9000,
# dragonfly:6379 and the cloud metadata endpoint.
MAX_REDIRECTS = 5
_CGNAT = ipaddress.ip_network("100.64.0.0/10")


def _is_public_ip(addr: str) -> bool:
    try:
        ip = ipaddress.ip_address(addr)
    except ValueError:
        return False
    if (ip.is_private or ip.is_loopback or ip.is_link_local or ip.is_multicast
            or ip.is_reserved or ip.is_unspecified):
        return False
    return not (ip.version == 4 and ip in _CGNAT)


def assert_public_url(url: str) -> None:
    """Raise ValueError unless url is http(s) and every address it resolves to is public."""
    p = urlparse(url)
    if p.scheme not in ("http", "https") or not p.hostname:
        raise ValueError(f"blocked non-http(s) url: {url!r}")
    port = p.port or (443 if p.scheme == "https" else 80)
    try:
        infos = socket.getaddrinfo(p.hostname, port, proto=socket.IPPROTO_TCP)
    except socket.gaierror as e:
        raise ValueError(f"cannot resolve {p.hostname!r}: {e}") from e
    for info in infos:
        addr = info[4][0]
        if not _is_public_ip(addr):
            raise ValueError(f"blocked internal address {addr} for host {p.hostname!r}")


def _get_checked(session, url, **kw):
    """GET with the SSRF check applied to the initial URL and to every redirect hop.

    requests follows 3xx transparently, which would let an allowed host bounce us
    to an internal address, so redirects are disabled and each Location is
    re-validated here instead.
    """
    kw.pop("allow_redirects", None)
    current = url
    for _ in range(MAX_REDIRECTS + 1):
        assert_public_url(current)
        r = session.get(current, allow_redirects=False, **kw)
        if r.status_code in (301, 302, 303, 307, 308):
            loc = r.headers.get("Location")
            if not loc:
                return r
            r.close()
            current = urljoin(current, loc)
            continue
        return r
    raise ValueError(f"too many redirects from {url!r}")


_display = None


def _ensure_display():
    # Start a virtual X display once so Chrome can run *headful* (not headless) in
    # the container — far less likely to be flagged by Cloudflare. On Windows/host
    # with a real display this is skipped.
    global _display
    if _display is not None or os.name == "nt" or os.environ.get("DISPLAY"):
        return
    try:
        from pyvirtualdisplay import Display

        _display = Display(visible=False, size=(1920, 1080))
        _display.start()
    except Exception as e:  # fall back to headless if xvfb is unavailable
        print(f"[scraper] xvfb unavailable ({e}); using headless", flush=True)


def get_driver():
    # uc=True → undetected-chromedriver (bypass Cloudflare Turnstile). On the host,
    # SCRAPER_HEADLESS=true runs headless (no popup window; matches a working manual
    # script). In-container it runs headful on the virtual display.
    _ensure_display()
    if os.getenv("SCRAPER_HEADLESS", "").lower() == "true":
        headless = True
    else:
        headless = os.name != "nt" and _display is None and not os.environ.get("DISPLAY")
    return Driver(uc=True, headless=headless)


def discover_chapters(driver, source_url: str, chapters_hint: str, existing: str = "") -> list[str]:
    """Return chapter URLs (natural order). If chapters_hint holds explicit URLs use
    them; if it is a 'A-B' range, filter discovered chapters to that range."""
    hint = (chapters_hint or "").strip()
    if "http" in hint:
        urls = re.findall(r"https?://\S+", hint)
        return sorted(set(urls), key=_chap_num)

    # Retry the index load — Cloudflare can challenge it ("Just a moment…") even on
    # the host; a single failed load must not fail the whole sync.
    urls = []
    for attempt in range(1, BATCH_RETRIES + 1):
        try:
            driver.uc_open_with_reconnect(source_url, reconnect_time=6)
            _pass_cloudflare(driver)
            driver.sleep(2)
            for _ in range(4):
                driver.execute_script("window.scrollBy(0, document.body.scrollHeight/4);")
                driver.sleep(0.6)
            urls = driver.execute_script(
                """
                var out = [];
                document.querySelectorAll('a[href*="-chap-"]').forEach(function(a){ out.push(a.href); });
                return Array.from(new Set(out));
                """
            ) or []
            title = driver.get_title()
            total_a = driver.execute_script("return document.querySelectorAll('a').length;")
        except Exception as e:
            title, total_a = f"error: {e}", -1
        print(f"[scraper] discover (try {attempt}): title={title!r} chap_links={len(urls)} total_a={total_a}", flush=True)
        if urls:
            break
        time.sleep(5)
    urls = sorted(set(urls), key=_chap_num)

    rng = re.match(r"^\s*(\d+)\s*-\s*(\d+)\s*$", hint)
    single = re.match(r"^\s*(\d+)\s*$", hint)
    if rng:
        lo, hi = float(rng.group(1)), float(rng.group(2))
        urls = [u for u in urls if lo <= _chap_num_f(u) <= hi]
    elif single:
        # a bare "158" = just that one chapter (was previously unmatched → scraped ALL)
        n = float(single.group(1))
        urls = [u for u in urls if _chap_num_f(u) == n]
    else:
        # Full sync (no explicit hint): skip chapters the comic already has so a
        # re-sync only DOWNLOADS new ones. `existing` is a comma list of chapter
        # labels that match the DB titles (chapter_label == the zip folder name).
        have = {e.strip() for e in (existing or "").split(",") if e.strip()}
        if have:
            urls = [u for u in urls if chapter_label(u) not in have]
    return urls


def _chap_num(url: str):
    m = re.search(r"chap-(\d+(?:[.-]\d+)?)", url)
    if not m:
        return (1e9, url)
    parts = re.split(r"[.-]", m.group(1))
    return tuple(int(p) for p in parts if p.isdigit()) or (1e9,)


def _chap_num_f(url: str) -> float:
    m = re.search(r"chap-(\d+)(?:[.-](\d+))?", url)
    if not m:
        return 1e9
    try:
        return float(m.group(1)) + (float(m.group(2)) / 100 if m.group(2) else 0.0)
    except Exception:
        return 1e9


def chapter_label(url: str) -> str:
    m = re.search(r"chap-(\d+(?:[.-]\d+)?)", url)
    return m.group(1).replace("-", ".") if m else url.rstrip("/").rsplit("/", 1)[-1]


# Collect every already-resolved image URL in the DOM. Used both by the lazy-load
# poll and as the final read, so "has another image resolved?" is exactly the same
# question as "what are we about to return?".
_COLLECT_JS = """
var urls = [];
document.querySelectorAll('.chapter_content .page-chapter img').forEach(function(img){
    var s = img.getAttribute('src') || img.getAttribute('data-cdn')
          || img.getAttribute('data-original') || img.getAttribute('data-src');
    if (s && s.indexOf('data:image') !== 0) urls.push(s);
});
return urls;
"""


def _challenged(driver) -> bool:
    # Cheap title check so the slow GUI solver below only runs on a real challenge.
    try:
        return "just a moment" in (driver.get_title() or "").lower()
    except Exception:
        return False


def _pass_cloudflare(driver):
    # SeleniumBase's Turnstile solver: clicks the "Verify you are human" checkbox via
    # the GUI (works on the xvfb display). Worth its ~2s only on an actual challenge —
    # once the driver holds a cf_clearance cookie the later chapters sail through, so
    # paying this on every chapter was ~2s of pure waste per chapter.
    if not _challenged(driver):
        return
    try:
        driver.uc_gui_click_captcha()
        driver.sleep(2)
    except Exception:
        pass


def _await_lazy_images(driver, timeout: float = LAZY_TIMEOUT) -> list[str]:
    """Scroll until the lazy-loaded image count stops growing, then return the URLs.

    Replaces a fixed 6×0.6s scroll that every chapter paid in full: most chapters
    settle in about a second, while a genuinely slow one is no longer truncated at
    3.6s. Needs two consecutive stable reads so a mid-load pause isn't mistaken
    for the end of the list.
    """
    deadline = time.time() + timeout
    urls: list[str] = []
    stable = 0
    while time.time() < deadline:
        try:
            driver.execute_script("window.scrollBy(0, document.body.scrollHeight/4);")
            found = driver.execute_script(_COLLECT_JS) or []
        except Exception:
            break
        if found and len(found) == len(urls):
            stable += 1
            if stable >= 2:
                return found
        else:
            stable = 0
        urls = found
        time.sleep(0.25)
    return urls


def get_image_links(driver, chapter_url: str, max_retries: int = 2) -> list[str]:
    for attempt in range(max_retries):
        try:
            # Chapter URLs come from the source page, so the Go guard never saw
            # them — check before Chrome navigates.
            assert_public_url(chapter_url)
            driver.uc_open_with_reconnect(chapter_url, reconnect_time=RECONNECT_TIME)
            _pass_cloudflare(driver)
            # No blind sleep + no separate wait_for_element_present: the poll below
            # scrolls first (some pages only mount the images on scroll) and blocks
            # only as long as the page actually needs.
            urls = _await_lazy_images(driver)
            if urls:
                return urls
            # Empty but no exception — retry rather than failing the whole batch.
            raise RuntimeError("no image urls in DOM")
        except Exception as e:
            try:
                title = driver.get_title()
            except Exception:
                title = "?"
            # A Cloudflare challenge shows titles like "Just a moment..." / "Attention Required".
            print(f"[scraper] chap load fail (try {attempt + 1}) title={title!r}: {e}", flush=True)
            time.sleep(3)
    return []


def _download_one(session, url, path, referer):
    headers = dict(DL_HEADERS, Referer=referer)
    for attempt in range(3):
        try:
            # SSRF-checked, including every redirect hop.
            r = _get_checked(session, url, headers=headers, stream=True, timeout=20)
            r.raise_for_status()
            r.raw.decode_content = True
            with open(path, "wb") as f:
                shutil.copyfileobj(r.raw, f)
            if os.path.getsize(path) > 0:
                return True
        except Exception:
            time.sleep(2)
    return False


def download_chapter(session, img_urls, chapter_dir, referer) -> int:
    os.makedirs(chapter_dir, exist_ok=True)
    tasks = []
    for i, url in enumerate(img_urls, start=1):
        p = os.path.join(chapter_dir, f"{i:03d}.jpg")
        if os.path.exists(p) and os.path.getsize(p) > 0:
            continue
        tasks.append((url, p))
    ok = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=DOWNLOAD_WORKERS) as ex:
        futs = {ex.submit(_download_one, session, u, p, referer): p for u, p in tasks}
        for fut in concurrent.futures.as_completed(futs):
            if fut.result():
                ok += 1
    return ok


def zip_dir(root: str) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_STORED) as z:  # images already compressed
        for dirpath, _dirs, files in os.walk(root):
            for name in sorted(files):
                full = os.path.join(dirpath, name)
                arc = os.path.relpath(full, root)
                z.write(full, arc)
    buf.seek(0)
    return buf.read()


BATCH_URL = CALLBACK_URL.replace("sync-callback", "sync-batch")
PROGRESS_URL = CALLBACK_URL.replace("sync-callback", "sync-progress")
FINALIZE_URL = CALLBACK_URL.replace("sync-callback", "sync-finalize")


def _post(url: str, payload: dict, timeout: int = 15):
    verify = os.getenv("SCRAPER_VERIFY_TLS", "true").lower() != "false"
    r = requests.post(url, json=payload, headers={"X-Internal-Secret": INTERNAL_SECRET}, timeout=timeout, verify=verify)
    r.raise_for_status()
    return r.json()


def request_batch(source_id: str):
    """Ask the api for a fresh import job for one batch → (import_id, upload_key)."""
    d = _post(BATCH_URL, {"source_id": source_id})
    return d["import_id"], d["upload_key"]


def report_progress(source_id: str, scraped: int, total: int):
    try:
        _post(PROGRESS_URL, {"source_id": source_id, "scraped": scraped, "total": total}, timeout=8)
    except Exception:
        pass


def callback(import_id: str, ok: bool, error: str = ""):
    # A batch's zip is uploaded (ok) → api imports it; or fail that batch's job.
    try:
        _post(CALLBACK_URL, {"import_id": import_id, "ok": ok, "error": error[:500]})
    except Exception as e:
        print(f"[scraper] batch callback failed for {import_id}: {e}", flush=True)


def finalize(source_id: str, ok: bool, failed: list[str] | None = None, note: str = ""):
    summary = note
    if not summary and failed:
        shown = ", ".join(failed[:20])
        summary = f"{len(failed)} chương lỗi: {shown}" + ("…" if len(failed) > 20 else "")
    try:
        _post(FINALIZE_URL, {"source_id": source_id, "ok": ok, "failed": summary})
    except Exception as e:
        print(f"[scraper] finalize failed for {source_id}: {e}", flush=True)


def _scrape_batch(driver, session, batch, referer, tmp):
    """Scrape one batch of chapter URLs into tmp/{label}. Retries the WHOLE batch up
    to BATCH_RETRIES if it yields zero images (e.g. a Cloudflare re-challenge).
    Progress is reported by the caller: with several workers in flight, "how many
    chapters are done" is a shared tally, not this batch's offset.
    Returns (got_any, failed_labels)."""
    labels = [chapter_label(u) for u in batch]
    for attempt in range(1, BATCH_RETRIES + 1):
        failed = []
        got_any = False
        for i, url in enumerate(batch):
            imgs = get_image_links(driver, url, max_retries=2)
            if imgs and download_chapter(session, imgs, os.path.join(tmp, labels[i]), referer) > 0:
                got_any = True
            else:
                failed.append(labels[i])
        if got_any:
            return True, failed  # some chapters ok; `failed` are individually-bad ones
        # whole batch empty → likely blocked; wipe partials and retry
        for d in list(os.listdir(tmp)):
            shutil.rmtree(os.path.join(tmp, d), ignore_errors=True)
        print(f"[scraper] batch {labels[0]}..{labels[-1]} empty — retry {attempt}/{BATCH_RETRIES}", flush=True)
        time.sleep(5)
    return False, labels  # every attempt failed → whole batch failed


# Cooperative cancellation: the api POSTs /cancel {source_id}; run_sync checks this
# before each chapter and stops (chapters already imported are kept).
_cancelled: set[str] = set()
_cancel_lock = threading.Lock()


def cancel_sync(source_id: str):
    with _cancel_lock:
        _cancelled.add(source_id)


def _is_cancelled(source_id: str) -> bool:
    with _cancel_lock:
        return source_id in _cancelled


def _clear_cancel(source_id: str):
    with _cancel_lock:
        _cancelled.discard(source_id)


def _batch_label(batch) -> str:
    if len(batch) == 1:
        return chapter_label(batch[0])
    return f"{chapter_label(batch[0])}..{chapter_label(batch[-1])}"


class _SyncState:
    """Shared tally for the workers scraping one source.

    Progress is a counter rather than a batch offset because batches no longer
    complete in order. `failed` is appended from both the scrape workers and the
    upload pool, hence the lock.
    """

    def __init__(self, source_id: str, total: int):
        self.source_id = source_id
        self.total = total
        self.done = 0
        self.failed: list[str] = []
        self._lock = threading.Lock()

    def advance(self, n: int, failed_labels: list[str]):
        with self._lock:
            self.done += n
            self.failed.extend(failed_labels)
            done = self.done
        report_progress(self.source_id, min(done, self.total), self.total)

    def mark_failed(self, labels: list[str]):
        with self._lock:
            self.failed.extend(labels)

    def snapshot(self) -> list[str]:
        with self._lock:
            return list(self.failed)


def _finish_batch(state: _SyncState, tmp: str, batch, lbl: str):
    """Zip → upload → hand to the api. Runs off the scrape thread so a worker's
    browser can start the next chapter instead of waiting on S3."""
    try:
        iid, key = request_batch(state.source_id)
        data = zip_dir(tmp)
        _s3().put_object(Bucket=S3_BUCKET, Key=key, Body=data, ContentType="application/zip")
        callback(iid, True)
        print(f"[scraper] {state.source_id}: chương {lbl} → import ({len(data)} bytes)", flush=True)
    except Exception as e:
        print(f"[scraper] {state.source_id}: chương {lbl} upload failed: {e}", flush=True)
        state.mark_failed([chapter_label(u) for u in batch])
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def _sync_worker(driver, jobs: "queue.Queue", state: _SyncState, referer: str, io_pool):
    """Pull batches until the queue drains or the sync is cancelled.

    `driver` may be the discovery browser lent to worker 0 — reusing it saves a
    browser start AND keeps the cf_clearance cookie it already earned. It is only
    lent: run_sync opened it and run_sync closes it.
    """
    own_driver = driver is None
    session = requests.Session()
    try:
        if own_driver:
            driver = get_driver()
            try:
                driver.set_page_load_timeout(45)
                driver.set_script_timeout(30)
            except Exception:
                pass
        while not _is_cancelled(state.source_id):
            try:
                batch = jobs.get_nowait()
            except queue.Empty:
                return
            lbl = _batch_label(batch)
            tmp = tempfile.mkdtemp(prefix="comic-sync-")
            handed_off = False
            try:
                got_any, batch_failed = _scrape_batch(driver, session, batch, referer, tmp)
                state.advance(len(batch), batch_failed)
                if got_any:
                    io_pool.submit(_finish_batch, state, tmp, batch, lbl)
                    handed_off = True  # _finish_batch owns tmp from here
                else:
                    print(f"[scraper] {state.source_id}: chương {lbl} FAILED after {BATCH_RETRIES} retries", flush=True)
            finally:
                if not handed_off:
                    shutil.rmtree(tmp, ignore_errors=True)
    finally:
        session.close()
        if own_driver and driver is not None:
            try:
                driver.quit()
            except Exception:
                pass


def run_sync(source_id: str, source_url: str, chapters_hint: str, existing: str = ""):
    """Parallel sync: discover once, then SYNC_WORKERS browsers pull batches from a
    shared queue (retry ×3 each) while an upload pool zips/ships finished batches.
    Chapters land out of order — the api derives a chapter's position from its title,
    so the reader still sees them in order. Failures are reported to the UI."""
    print(f"[scraper] sync source {source_id} start: {source_url}", flush=True)
    try:
        assert_public_url(source_url)
    except ValueError as e:
        print(f"[scraper] refusing source {source_id}: {e}", flush=True)
        finalize(source_id, False, note=f"nguồn bị chặn: {e}")
        return
    _clear_cancel(source_id)  # a fresh sync must not inherit a stale cancel flag
    lead = None
    try:
        lead = get_driver()
        try:
            lead.set_page_load_timeout(45)
            lead.set_script_timeout(30)
        except Exception:
            pass
        chapters = discover_chapters(lead, source_url, chapters_hint, existing)
        if not chapters:
            finalize(source_id, False, note="không tìm thấy chương nào ở nguồn")
            return
        total = len(chapters)
        jobs: queue.Queue = queue.Queue()
        for start in range(0, total, BATCH_SIZE):
            jobs.put(chapters[start:start + BATCH_SIZE])
        workers = max(1, min(SYNC_WORKERS, jobs.qsize()))
        print(f"[scraper] {source_id}: {total} chapters, batch={BATCH_SIZE}, workers={workers}", flush=True)
        report_progress(source_id, 0, total)

        state = _SyncState(source_id, total)
        referer = _referer(source_url)
        # Uploads are network-bound and cheap next to a browser, so one lane per
        # worker plus a spare keeps a slow S3 put from stalling a scrape thread.
        io_pool = concurrent.futures.ThreadPoolExecutor(max_workers=workers + 1, thread_name_prefix="upload")
        try:
            with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
                # Worker 0 borrows the discovery browser; the rest start their own.
                runs = [pool.submit(_sync_worker, lead if i == 0 else None, jobs, state, referer, io_pool)
                        for i in range(workers)]
                for r in runs:
                    r.result()  # re-raise a worker crash into the handler below
        finally:
            # shutdown(wait=True) drains every submitted upload, so by the time this
            # returns state.failed has heard from the upload lane too.
            io_pool.shutdown(wait=True)

        failed = state.snapshot()
        if _is_cancelled(source_id):  # on cancel the api already set status='cancelled'
            print(f"[scraper] {source_id}: ĐÃ NGỪNG ở {state.done}/{total} (giữ chương đã import)", flush=True)
        else:
            finalize(source_id, ok=(len(failed) < total), failed=failed)
            print(f"[scraper] {source_id}: DONE ({total - len(failed)}/{total} chương ok, {len(failed)} lỗi)", flush=True)
    except Exception as e:
        print(f"[scraper] {source_id} ERROR: {e}", flush=True)
        finalize(source_id, False, note=str(e)[:200])
    finally:
        _clear_cancel(source_id)
        if lead is not None:
            try:
                lead.quit()
            except Exception:
                pass
