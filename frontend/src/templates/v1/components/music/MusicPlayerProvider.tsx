"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { isPlayable, trackAudioURL, type Track } from "@/lib/music";

/**
 * The music vertical's playback engine.
 *
 * Owns exactly ONE `<audio>` element for the whole app, mounted in `MasterBase`
 * above the router outlet — that placement is the point: playback survives
 * navigation, so starting a track in the library and then walking to the
 * newsfeed keeps it playing. A per-view `<audio>` would restart on every route
 * change.
 *
 * Everything that renders playback UI (`NowPlayingBar`, `MusicWidget`, the
 * library views) reads this context rather than holding audio state of its own,
 * so "what is playing" has a single source of truth and the highlighted row is
 * always the row you actually hear.
 *
 * Deliberately NOT persisted: the queue lives in memory only. A reload stops the
 * music, which matches every other player people use in a browser tab.
 */

export type RepeatMode = "off" | "one" | "all";

interface MusicPlayerState {
  /** The active queue, in play order. Empty when nothing has been started. */
  queue: Track[];
  /** Index into `queue`, or -1 when idle. */
  index: number;
  current: Track | null;
  playing: boolean;
  /** Seconds. 0 until the browser reports metadata. */
  position: number;
  /** Seconds. 0 when unknown (streaming source with no reported duration). */
  duration: number;
  /** 0-100, derived — what the progress bar renders. */
  progressPct: number;
  shuffle: boolean;
  repeat: RepeatMode;
  /** Set when the browser refuses to play (autoplay policy, decode, 404). */
  error: string | null;
}

interface MusicPlayerActions {
  /** Replace the queue and start at `startIndex` (default 0). */
  playQueue: (tracks: Track[], startIndex?: number) => void;
  /** Play a single track, replacing the queue with just it. */
  playTrack: (track: Track) => void;
  /** Toggle play/pause on the current track. No-op when idle. */
  toggle: () => void;
  next: () => void;
  prev: () => void;
  /** Seek to a percentage (0-100) of the current duration. */
  seekPct: (pct: number) => void;
  toggleShuffle: () => void;
  cycleRepeat: () => void;
  /** Stop playback and clear the queue — hides the bar. */
  stop: () => void;
  /** True when this track is the one currently loaded (playing or paused). */
  isCurrent: (trackId: string) => boolean;
}

type MusicPlayerContextValue = MusicPlayerState & MusicPlayerActions;

const MusicPlayerContext = createContext<MusicPlayerContextValue | null>(null);

/**
 * Read the player. Returns `null` outside a provider rather than throwing, so a
 * component can be rendered standalone (design-system preview cards do exactly
 * that) without blanking the card.
 */
export function useMusicPlayerOptional(): MusicPlayerContextValue | null {
  return useContext(MusicPlayerContext);
}

/** Read the player, asserting a provider is mounted. */
export function useMusicPlayer(): MusicPlayerContextValue {
  const ctx = useContext(MusicPlayerContext);
  if (!ctx) {
    throw new Error("useMusicPlayer must be used inside <MusicPlayerProvider>");
  }
  return ctx;
}

/**
 * Fisher-Yates over the tracks themselves, keeping the picked track at
 * position 0 — shuffling from a chosen row should still play THAT row first.
 */
function shuffledFrom(list: Track[], firstIndex: number): Track[] {
  const first = list[firstIndex];
  const rest = list.filter((_, i) => i !== firstIndex);
  for (let i = rest.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    const a = rest[i];
    const b = rest[j];
    if (a !== undefined && b !== undefined) {
      rest[i] = b;
      rest[j] = a;
    }
  }
  return first ? [first, ...rest] : rest;
}

export function MusicPlayerProvider({ children }: { children: ReactNode }) {
  const audioRef = useRef<HTMLAudioElement | null>(null);

  const [queue, setQueue] = useState<Track[]>([]);
  const [index, setIndex] = useState(-1);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [shuffle, setShuffle] = useState(false);
  const [repeat, setRepeat] = useState<RepeatMode>("off");
  const [error, setError] = useState<string | null>(null);

  const current = index >= 0 && index < queue.length ? queue[index] ?? null : null;

  // The `ended` handler is registered once, in an effect with no deps, so it
  // must not close over `repeat`/`queue.length` directly — these refs give it
  // live values without re-binding the listeners on every state change.
  const repeatRef = useRef(repeat);
  repeatRef.current = repeat;
  const queueLenRef = useRef(0);
  queueLenRef.current = queue.length;

  const play = useCallback(() => {
    const el = audioRef.current;
    if (!el) return;
    const p = el.play();
    if (p && typeof p.catch === "function") {
      p.catch(() => {
        setPlaying(false);
        setError("Playback was blocked. Press play to start.");
      });
    }
  }, []);

  const playQueue = useCallback((tracks: Track[], startIndex = 0) => {
    const playable = tracks.filter(isPlayable);
    if (playable.length === 0) {
      setError("That track has no audio file attached yet.");
      return;
    }
    // Map the requested index through the filter, then through shuffle order.
    const wanted = tracks[startIndex];
    const mapped = wanted ? Math.max(0, playable.findIndex((t) => t.id === wanted.id)) : 0;
    setError(null);
    if (shuffle) {
      setQueue(shuffledFrom(playable, mapped));
      setIndex(0);
    } else {
      setQueue(playable);
      setIndex(mapped);
    }
    setPlaying(true);
  }, [shuffle]);

  const playTrack = useCallback((track: Track) => {
    if (!isPlayable(track)) {
      setError("That track has no audio file attached yet.");
      return;
    }
    setError(null);
    setQueue([track]);
    setIndex(0);
    setPlaying(true);
  }, []);

  const toggle = useCallback(() => {
    const el = audioRef.current;
    if (!el || !current) return;
    setError(null);
    if (el.paused) {
      setPlaying(true);
      play();
    } else {
      el.pause();
      setPlaying(false);
    }
  }, [current, play]);

  const next = useCallback(() => {
    setIndex((i) => {
      if (queue.length === 0) return i;
      if (i + 1 < queue.length) return i + 1;
      return repeatRef.current === "all" ? 0 : i;
    });
  }, [queue.length]);

  const prev = useCallback(() => {
    const el = audioRef.current;
    // Standard player behaviour: past ~3s, "previous" restarts the track.
    if (el && el.currentTime > 3) {
      el.currentTime = 0;
      setPosition(0);
      return;
    }
    setIndex((i) => {
      if (queue.length === 0) return i;
      if (i - 1 >= 0) return i - 1;
      return repeatRef.current === "all" ? queue.length - 1 : i;
    });
  }, [queue.length]);

  const seekPct = useCallback((pct: number) => {
    const el = audioRef.current;
    if (!el || !Number.isFinite(el.duration) || el.duration <= 0) return;
    const clamped = Math.min(100, Math.max(0, pct));
    el.currentTime = (clamped / 100) * el.duration;
    setPosition(el.currentTime);
  }, []);

  const stop = useCallback(() => {
    const el = audioRef.current;
    if (el) el.pause();
    setPlaying(false);
    setQueue([]);
    setIndex(-1);
    setPosition(0);
    setDuration(0);
    setError(null);
  }, []);

  const toggleShuffle = useCallback(() => setShuffle((s) => !s), []);
  const cycleRepeat = useCallback(
    () => setRepeat((r) => (r === "off" ? "all" : r === "all" ? "one" : "off")),
    [],
  );

  const isCurrent = useCallback((trackId: string) => current?.id === trackId, [current]);

  // Load the source whenever the current track changes, and autoplay if we were
  // already in a playing state (i.e. the user pressed play, then skipped).
  useEffect(() => {
    const el = audioRef.current;
    if (!el || !current?.audio_asset_id) return;
    el.src = trackAudioURL(current.audio_asset_id);
    el.load();
    setPosition(0);
    setDuration(0);
    if (playing) play();
    // `playing` is intentionally omitted: this effect is about the SOURCE
    // changing. Play/pause on an unchanged source is handled by `toggle`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id, current?.audio_asset_id, play]);

  // Wire the media element's events to state, once.
  useEffect(() => {
    const el = audioRef.current;
    if (!el) return;

    const onTime = () => setPosition(el.currentTime);
    const onMeta = () => setDuration(Number.isFinite(el.duration) ? el.duration : 0);
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onError = () => {
      setPlaying(false);
      setError("This track could not be played.");
    };
    const onEnded = () => {
      if (repeatRef.current === "one") {
        el.currentTime = 0;
        void el.play();
        return;
      }
      setIndex((i) => {
        if (i + 1 < queueLenRef.current) return i + 1;
        if (repeatRef.current === "all") return 0;
        setPlaying(false);
        return i;
      });
    };

    el.addEventListener("timeupdate", onTime);
    el.addEventListener("loadedmetadata", onMeta);
    el.addEventListener("durationchange", onMeta);
    el.addEventListener("play", onPlay);
    el.addEventListener("pause", onPause);
    el.addEventListener("error", onError);
    el.addEventListener("ended", onEnded);
    return () => {
      el.removeEventListener("timeupdate", onTime);
      el.removeEventListener("loadedmetadata", onMeta);
      el.removeEventListener("durationchange", onMeta);
      el.removeEventListener("play", onPlay);
      el.removeEventListener("pause", onPause);
      el.removeEventListener("error", onError);
      el.removeEventListener("ended", onEnded);
    };
  }, []);

  const progressPct = duration > 0 ? Math.min(100, (position / duration) * 100) : 0;

  const value = useMemo<MusicPlayerContextValue>(
    () => ({
      queue,
      index,
      current,
      playing,
      position,
      duration,
      progressPct,
      shuffle,
      repeat,
      error,
      playQueue,
      playTrack,
      toggle,
      next,
      prev,
      seekPct,
      toggleShuffle,
      cycleRepeat,
      stop,
      isCurrent,
    }),
    [
      queue, index, current, playing, position, duration, progressPct, shuffle, repeat, error,
      playQueue, playTrack, toggle, next, prev, seekPct, toggleShuffle, cycleRepeat, stop, isCurrent,
    ],
  );

  return (
    <MusicPlayerContext.Provider value={value}>
      {children}
      {/* One element for the whole app. `preload="metadata"` so the duration
          shows before the user commits to downloading the file. */}
      <audio ref={audioRef} preload="metadata" hidden />
    </MusicPlayerContext.Provider>
  );
}
