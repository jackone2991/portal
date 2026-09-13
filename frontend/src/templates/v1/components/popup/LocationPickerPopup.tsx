"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Modal, BtnPrimary, BtnSecondary, inputCls, inputStyle } from "./Modal";
import { Icon } from "../ui/Icon";
import {
  MAX_ZOOM,
  MIN_ZOOM,
  TILE,
  coordName,
  latToTileY,
  lonToTileX,
  searchLocations,
  tileURL,
  tileXToLon,
  tileYToLat,
  type Location,
  type LocationResult,
} from "@/lib/geo";

/**
 * "Add Location" picker for the newsfeed composer — search a location, or drop the
 * pin anywhere on the map.
 *
 * The map is a hand-rolled slippy map: a grid of 256px OpenStreetMap tiles
 * positioned by the Web-Mercator formulas in `lib/geo.ts`. No map library is
 * installed and none is added — the whole interaction (pan, zoom, click-to-pin)
 * is pixel arithmetic, so it costs nothing in the bundle. Attribution is
 * required by the OSM tile usage policy and is rendered in the corner.
 *
 * The name stays editable after picking: the search gives "Hoàn Kiếm", but the
 * post usually wants "quán cà phê ở Hoàn Kiếm". A pin dropped on bare map has
 * no name to look up (the geocoder is forward-only), so it falls back to its
 * coordinates rather than inventing a label.
 */
const DEFAULT_CENTER = { lat: 21.0278, lon: 105.8342 }; // Hà Nội
const MAP_H = 300;

export function LocationPickerPopup({
  open,
  onClose,
  onPick,
  initial,
}: {
  open: boolean;
  onClose: () => void;
  onPick: (location: Location) => void;
  initial?: Location | null;
}) {
  const [center, setCenter] = useState(initial ?? DEFAULT_CENTER);
  const [zoom, setZoom] = useState(initial ? 15 : 12);
  const [pin, setPin] = useState<{ lat: number; lon: number } | null>(initial ?? null);
  const [name, setName] = useState(initial?.name ?? "");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<LocationResult[]>([]);
  const [searching, setSearching] = useState(false);

  const boxRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(560);

  // Tile layout needs the real pixel width; the modal is responsive.
  useEffect(() => {
    if (!open) return;
    const measure = () => setWidth(boxRef.current?.clientWidth ?? 560);
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [open]);

  useEffect(() => {
    if (open) return;
    setQuery("");
    setResults([]);
  }, [open]);

  // Debounced search — one in-flight request, aborted when the query moves on.
  useEffect(() => {
    if (!open) return;
    const q = query.trim();
    if (q.length < 2) {
      setResults([]);
      setSearching(false);
      return;
    }
    const ctrl = new AbortController();
    setSearching(true);
    const t = setTimeout(async () => {
      const found = await searchLocations(q, ctrl.signal);
      if (!ctrl.signal.aborted) {
        setResults(found);
        setSearching(false);
      }
    }, 350);
    return () => {
      clearTimeout(t);
      ctrl.abort();
    };
  }, [query, open]);

  /* ── projection ─────────────────────────────────────────────────── */

  const originX = lonToTileX(center.lon, zoom) * TILE - width / 2;
  const originY = latToTileY(center.lat, zoom) * TILE - MAP_H / 2;
  const worldTiles = 2 ** zoom;

  const tiles: { key: string; url: string; left: number; top: number }[] = [];
  for (let tx = Math.floor(originX / TILE); tx <= Math.floor((originX + width) / TILE); tx += 1) {
    for (
      let ty = Math.max(0, Math.floor(originY / TILE));
      ty <= Math.min(worldTiles - 1, Math.floor((originY + MAP_H) / TILE));
      ty += 1
    ) {
      tiles.push({
        key: `${zoom}/${tx}/${ty}`,
        url: tileURL(zoom, tx, ty),
        left: tx * TILE - originX,
        top: ty * TILE - originY,
      });
    }
  }

  const pinPos = pin
    ? {
        left: lonToTileX(pin.lon, zoom) * TILE - originX,
        top: latToTileY(pin.lat, zoom) * TILE - originY,
      }
    : null;

  const panBy = useCallback(
    (dx: number, dy: number) => {
      setCenter((c) => {
        const cx = lonToTileX(c.lon, zoom) * TILE - dx;
        const cy = latToTileY(c.lat, zoom) * TILE - dy;
        return { lat: tileYToLat(cy / TILE, zoom), lon: tileXToLon(cx / TILE, zoom) };
      });
    },
    [zoom],
  );

  /* ── pointer: drag to pan, click (no drag) to drop the pin ──────── */

  const drag = useRef<{ id: number; moved: number } | null>(null);

  function onPointerDown(e: React.PointerEvent<HTMLDivElement>) {
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    drag.current = { id: e.pointerId, moved: 0 };
  }

  function onPointerMove(e: React.PointerEvent<HTMLDivElement>) {
    if (drag.current?.id !== e.pointerId) return;
    drag.current.moved += Math.abs(e.movementX) + Math.abs(e.movementY);
    panBy(e.movementX, e.movementY);
  }

  function onPointerUp(e: React.PointerEvent<HTMLDivElement>) {
    const d = drag.current;
    drag.current = null;
    if (d?.id !== e.pointerId) return;
    if (d.moved > 4) return; // that was a pan, not a pick
    const rect = e.currentTarget.getBoundingClientRect();
    const lat = tileYToLat((originY + (e.clientY - rect.top)) / TILE, zoom);
    const lon = tileXToLon((originX + (e.clientX - rect.left)) / TILE, zoom);
    setPin({ lat, lon });
    setName((n) => (n.trim() ? n : coordName(lat, lon)));
  }

  function choose(p: LocationResult) {
    setPin({ lat: p.lat, lon: p.lon });
    setCenter({ lat: p.lat, lon: p.lon });
    setZoom(14);
    setName(p.name);
    setResults([]);
    setQuery("");
  }

  const resolved: Location | null = pin
    ? { name: name.trim() || coordName(pin.lat, pin.lon), lat: pin.lat, lon: pin.lon }
    : null;

  return (
    <Modal open={open} onClose={onClose} title="Add Location" width={640}>
      <div className="space-y-3 p-6 pb-4">
        <div className="relative">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Tìm thành phố, quận, địa danh…"
            className={`${inputCls} pl-9`}
            style={inputStyle}
          />
          <span
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2"
            style={{ color: "var(--tpl-muted)" }}
          >
            <Icon name="magnifying-glass-icon" size={14} />
          </span>

          {(searching || results.length > 0) && (
            <ul
              className="absolute left-0 right-0 top-11 z-10 max-h-56 overflow-y-auto rounded-lg py-1 shadow-lg"
              style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
            >
              {searching && results.length === 0 ? (
                <li className="px-4 py-2 text-sm" style={{ color: "var(--tpl-muted)" }}>
                  Đang tìm…
                </li>
              ) : (
                results.map((r) => (
                  <li key={`${r.lat},${r.lon},${r.name}`}>
                    <button
                      type="button"
                      onClick={() => choose(r)}
                      className="block w-full px-4 py-2 text-left transition hover:bg-[var(--tpl-surface-2)]"
                    >
                      <span
                        className="block text-sm font-medium"
                        style={{ color: "var(--tpl-heading)" }}
                      >
                        {r.name}
                      </span>
                      {r.detail && (
                        <span className="block text-xs" style={{ color: "var(--tpl-muted)" }}>
                          {r.detail}
                        </span>
                      )}
                    </button>
                  </li>
                ))
              )}
            </ul>
          )}
        </div>

        <div
          ref={boxRef}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          className="relative touch-none select-none overflow-hidden rounded-lg border"
          style={{
            height: MAP_H,
            borderColor: "var(--tpl-border)",
            background: "var(--tpl-surface-2)",
            cursor: "crosshair",
          }}
        >
          {tiles.map((t) => (
            /* eslint-disable-next-line @next/next/no-img-element -- OSM raster tile, not a static/optimizable asset */
            <img
              key={t.key}
              src={t.url}
              alt=""
              draggable={false}
              width={TILE}
              height={TILE}
              className="pointer-events-none absolute"
              style={{ left: t.left, top: t.top }}
            />
          ))}

          {pinPos && (
            <span
              className="pointer-events-none absolute grid place-items-center rounded-full text-white shadow"
              style={{
                left: pinPos.left - 14,
                top: pinPos.top - 28,
                width: 28,
                height: 28,
                background: "var(--tpl-accent)",
              }}
            >
              <Icon name="small-pin-icon" size={14} />
            </span>
          )}

          <div className="absolute right-2 top-2 flex flex-col gap-1">
            <ZoomBtn label="Zoom in" onClick={() => setZoom((z) => Math.min(MAX_ZOOM, z + 1))}>
              +
            </ZoomBtn>
            <ZoomBtn label="Zoom out" onClick={() => setZoom((z) => Math.max(MIN_ZOOM, z - 1))}>
              −
            </ZoomBtn>
          </div>

          <a
            href="https://www.openstreetmap.org/copyright"
            target="_blank"
            rel="noreferrer noopener"
            onPointerDown={(e) => e.stopPropagation()}
            className="absolute bottom-0 right-0 bg-white/80 px-1.5 py-0.5 text-[10px]"
            style={{ color: "var(--tpl-muted)" }}
          >
            © OpenStreetMap contributors
          </a>
        </div>

        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          Tìm ở ô trên, hoặc bấm thẳng lên bản đồ để thả ghim. Kéo để di chuyển.
        </p>

        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Tên địa điểm hiển thị trên bài viết"
          className={inputCls}
          style={inputStyle}
          disabled={!pin}
        />
      </div>

      <div
        className="flex items-center justify-between gap-2 border-t px-6 py-4"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        <span className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
          {pin ? coordName(pin.lat, pin.lon) : "Chưa chọn địa điểm"}
        </span>
        <span className="flex gap-2">
          <BtnSecondary onClick={onClose}>Cancel</BtnSecondary>
          <BtnPrimary
            disabled={!resolved}
            onClick={() => {
              if (resolved) {
                onPick(resolved);
                onClose();
              }
            }}
          >
            Add Location
          </BtnPrimary>
        </span>
      </div>
    </Modal>
  );
}

function ZoomBtn({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      onPointerDown={(e) => e.stopPropagation()}
      onClick={onClick}
      className="grid h-7 w-7 place-items-center rounded-md text-base font-bold shadow"
      style={{ background: "var(--tpl-surface)", color: "var(--tpl-heading)" }}
    >
      {children}
    </button>
  );
}
