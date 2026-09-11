"use client";

// Server-side zip chapter import (SPEC-02 P1.7). The browser only: creates the
// import job, PUTs the .zip to the API (which stores it + enqueues the worker),
// then polls the persisted job until done/failed. All unpacking, image ingest and
// page creation happen on the worker.

import { api, baseURL } from "./api-client";

export type ImportStatus = "pending" | "uploaded" | "processing" | "done" | "failed";
export interface ImportFileResult { name: string; ok: boolean; error?: string }
export interface ImportJob {
  id: string;
  comic_id: string;
  chapter_id: string | null;
  status: ImportStatus;
  total: number;
  succeeded: number;
  failed: number;
  report: ImportFileResult[];
  error?: string;
  created_at: string;
  updated_at: string;
}

/** Chapter-level import job: a flat zip of images → pages of this chapter. */
export async function createImport(chapterId: string): Promise<ImportJob> {
  return api<ImportJob>(`/api/v1/chapters/${chapterId}/imports`, { method: "POST" });
}
/** Comic-level import job: a multi-chapter zip (folder = chapter) → chapters + pages. */
export async function createComicImport(comicId: string): Promise<ImportJob> {
  return api<ImportJob>(`/api/v1/comics/${comicId}/imports`, { method: "POST" });
}
export async function getImport(importId: string): Promise<ImportJob> {
  return api<ImportJob>(`/api/v1/imports/${importId}`);
}

export interface RunImportProgress {
  phase: "uploading" | "processing";
  uploadPct?: number;
  job?: ImportJob;
}

/** Import a flat zip into an existing chapter. */
export function runZipImport(chapterId: string, file: File, onProgress: (p: RunImportProgress) => void): Promise<ImportJob> {
  return runImport(() => createImport(chapterId), file, onProgress);
}
/** Import a whole comic (multi-chapter zip). */
export function runComicZipImport(comicId: string, file: File, onProgress: (p: RunImportProgress) => void): Promise<ImportJob> {
  return runImport(() => createComicImport(comicId), file, onProgress);
}

/** create job → upload zip (progress) → poll to a terminal state. */
async function runImport(createJob: () => Promise<ImportJob>, file: File, onProgress: (p: RunImportProgress) => void): Promise<ImportJob> {
  const job = await createJob();
  onProgress({ phase: "uploading", uploadPct: 0 });
  await putZip(job.id, file, (pct) => onProgress({ phase: "uploading", uploadPct: pct }));

  onProgress({ phase: "processing" });
  let last: ImportJob = job;
  for (let i = 0; i < 3600; i += 1) { // long ceiling — whole-comic imports take a while
    await sleep(2000);
    last = await getImport(job.id);
    onProgress({ phase: "processing", job: last });
    if (last.status === "done" || last.status === "failed") return last;
  }
  return last;
}

function putZip(importId: string, file: File, onProgress: (pct: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", `${baseURL}/api/v1/imports/${importId}/zip`);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Content-Type", "application/zip");
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100)); };
    // Surface the API's problem+json `detail` — the status alone hid real reasons
    // (e.g. the zip being over the size cap) behind a bare number.
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`Tải zip lỗi (${xhr.status})${zipErrDetail(xhr.responseText)}.`)));
    xhr.onerror = () => reject(new Error("Lỗi mạng khi tải zip."));
    xhr.send(file);
  });
}

/** `: <detail>` from an RFC 7807 body, or "" when there is nothing readable. */
function zipErrDetail(body: string): string {
  try {
    const detail = (JSON.parse(body) as { detail?: string }).detail;
    return detail ? `: ${detail}` : "";
  } catch {
    return "";
  }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
