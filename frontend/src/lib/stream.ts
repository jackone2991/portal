// Data layer for the life-stream home (SPEC-06). The merged timeline read.

import { api } from "./api-client";

export interface StreamItem {
  id: string;
  source_module: string; // journal | media | bank | comic | people
  event_type: string;
  occurred_at: string;
  // journal items
  body_md?: string | null;
  mood?: string | null;
  // system items
  title?: string | null;
  href?: string | null;
}

export interface StreamPage {
  items: StreamItem[];
  next_cursor?: string | null;
}

export async function getStream(cursor?: string): Promise<StreamPage> {
  const q = cursor ? `?cursor=${cursor}` : "";
  const r = await api<StreamPage>(`/api/v1/stream${q}`);
  return { items: r.items ?? [], next_cursor: r.next_cursor };
}
