"use client";

// Image upload helper (SPEC-01 media pipeline), reused by the comic manager for
// covers + pages. Mirrors the video flow in UploadStudio: create an asset session
// → PUT the original (API-proxied, dev) → complete (sniff + enqueue) → poll until
// the worker's WebP variants are ready. Returns the ready asset id.

import { baseURL } from "./api-client";

interface AssetJSON {
  id: string;
  status: string;
  width: number | null;
  height: number | null;
  error?: string;
}

export interface UploadedImage {
  assetId: string;
  width: number | null;
  height: number | null;
}

export async function uploadImage(file: File, onProgress?: (pct: number) => void): Promise<UploadedImage> {
  const contentType = file.type || guessType(file.name);
  if (!contentType.startsWith("image/")) throw new Error("Chỉ chấp nhận tệp ảnh.");

  // 1. create the asset + upload session
  const cr = await fetch(`${baseURL}/api/v1/assets`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ filename: file.name, content_type: contentType, size_bytes: file.size }),
  });
  if (!cr.ok) throw new Error("Không tạo được phiên tải lên.");
  const { asset } = (await cr.json()) as { asset: AssetJSON };

  // 2. upload the original (proxied through the API, with progress)
  await putWithProgress(`${baseURL}/api/v1/assets/${asset.id}/source`, file, contentType, onProgress);

  // 3. confirm → enqueue variant processing
  const co = await fetch(`${baseURL}/api/v1/assets/${asset.id}/complete`, { method: "POST", credentials: "include" });
  if (!co.ok) throw new Error("Không hoàn tất được tải lên.");

  // 4. poll until the worker finishes (variants ready)
  const done = await poll(asset.id);
  if (done.status !== "ready") throw new Error(done.error || "Xử lý ảnh thất bại.");
  return { assetId: done.id, width: done.width, height: done.height };
}

function guessType(name: string): string {
  const ext = name.toLowerCase().split(".").pop() ?? "";
  return { jpg: "image/jpeg", jpeg: "image/jpeg", png: "image/png", webp: "image/webp", gif: "image/gif", avif: "image/avif" }[ext] ?? "application/octet-stream";
}

function putWithProgress(url: string, file: File, contentType: string, onProgress?: (pct: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.withCredentials = true; // send the session cookie cross-subdomain
    xhr.setRequestHeader("Content-Type", contentType);
    xhr.upload.onprogress = (e) => { if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100)); };
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`Tải lên lỗi (${xhr.status}).`)));
    xhr.onerror = () => reject(new Error("Lỗi mạng khi tải lên."));
    xhr.send(file);
  });
}

async function poll(id: string): Promise<AssetJSON> {
  for (let i = 0; i < 120; i += 1) {
    const r = await fetch(`${baseURL}/api/v1/assets/${id}`, { credentials: "include" });
    if (r.ok) {
      const a = (await r.json()) as AssetJSON;
      if (a.status === "ready" || a.status === "failed") return a;
    }
    await new Promise((res) => setTimeout(res, 1500));
  }
  return { id, status: "failed", width: null, height: null, error: "Hết thời gian chờ xử lý ảnh." };
}
