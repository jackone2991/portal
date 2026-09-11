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
  /**
   * Jump straight to a position in the CURRENT queue and play it — what the
   * queue popup does when you click a row.
   *
   * The index is into `queue`, which is already in play order (shuffled if
   * shuffle is on), so what the list shows is what this addresses. Rebuilding
   * the queue through playQueue instead would re-roll the shuffle and move every
   * other row out from under the click.
   */
  jumpTo: (index: number) => void;
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
  // The queue as it was handed to playQueue, before any shuffle. Kept so that
  // turning shuffle back off restores the real order instead of leaving the
  // listener stuck in a random one.
  const [baseQueue, setBaseQueue] = useState<Track[]>([]);
  const [index, setIndex] = useState(-1);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [shuffle, setShuffle] = useState(false);
  const [repeat, setRepeat] = useState<RepeatMode>("off");
  const [error, setError] = useState<string | null>(null);
  // Bumped by every explicit play request. The source effect below keys off it
  // as well as the track id, so asking to play the track that is ALREADY loaded
  // still reloads and restarts it — see the effect for why that matters.
  const [playToken, setPlayToken] = useState(0);

  const current = index >= 0 && index < queue.length ? queue[index] ?? null : null;

  // The `ended` handler is registered once, in an effect with no deps, so it
  // must not close over `repeat`/`queue.length` directly — these refs give it
  // live values without re-binding the listeners on every state change.
  const repeatRef = useRef(repeat);
  repeatRef.current = repeat;
  const queueLenRef = useRef(0);
  queueLenRef.current = queue.length;
  // A seek requested before the duration was known, replayed once it is.
  const pendingSeekRef = useRef<number | null>(null);

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
    setBaseQueue(playable);
    if (shuffle) {
      setQueue(shuffledFrom(playable, mapped));
      setIndex(0);
    } else {
      setQueue(playable);
      setIndex(mapped);
    }
    setPlaying(true);
    setPlayToken((t) => t + 1);
  }, [shuffle]);

  const jumpTo = useCallback((to: number) => {
    // Bounds come from the ref, not from a setQueue updater. A state updater has
    // to be pure — React may run it twice, or discard it — so the setIndex that
    // performs the jump cannot live inside one. It silently did nothing when it
    // did.
    if (to < 0 || to >= queueLenRef.current) return; // stale click on a moved queue
    setError(null);
    setIndex(to);
    setPlaying(true);
    setPlayToken((t) => t + 1);
  }, []);

  const playTrack = useCallback((track: Track) => {
    if (!isPlayable(track)) {
      setError("That track has no audio file attached yet.");
      return;
    }
    setError(null);
    setQueue([track]);
    setBaseQueue([track]);
    setIndex(0);
    setPlaying(true);
    setPlayToken((t) => t + 1);
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

  /**
   * Seek to a percentage of the track.
   *
   * A scrub that arrives before the browser knows the duration used to be
   * dropped on the floor. That is a real window — `load()` resets duration to
   * NaN, and the click that follows an auto-advance often lands inside it — and
   * silently ignoring it makes the progress bar look broken. The request is
   * parked instead and applied on `loadedmetadata`.
   */
  const seekPct = useCallback((pct: number) => {
    const clamped = Math.min(100, Math.max(0, pct));
    const el = audioRef.current;
    if (!el || !Number.isFinite(el.duration) || el.duration <= 0) {
      pendingSeekRef.current = clamped;
      return;
    }
    pendingSeekRef.current = null;
    el.currentTime = (clamped / 100) * el.duration;
    setPosition(el.currentTime);
  }, []);

  const stop = useCallback(() => {
    const el = audioRef.current;
    if (el) el.pause();
    setPlaying(false);
    setQueue([]);
    setBaseQueue([]);
    setIndex(-1);
    setPosition(0);
    setDuration(0);
    setError(null);
  }, []);

  /**
   * Shuffle reorders the queue that is playing RIGHT NOW.
   *
   * It used to only set a flag that the next `playQueue` would read, so pressing
   * it mid-listen lit the button up and changed nothing — the definition of a
   * control that does not work.
   *
   * The current track stays put and keeps playing: only the tracks around it are
   * reordered, and `index` is re-pointed at it. Because `current.id` does not
   * change, the source effect does not re-run and playback is not interrupted.
   */
  const toggleShuffle = useCallback(() => {
    const on = !shuffle;
    setShuffle(on);

    const base = baseQueue.length > 0 ? baseQueue : queue;
    if (!current || base.length === 0) return;

    const next = on ? shuffledFrom(base, base.findIndex((t) => t.id === current.id)) : base;
    const at = next.findIndex((t) => t.id === current.id);
    setQueue(next);
    setIndex(at >= 0 ? at : 0);
  }, [shuffle, baseQueue, queue, current]);
  const cycleRepeat = useCallback(
    () => setRepeat((r) => (r === "off" ? "all" : r === "all" ? "one" : "off")),
    [],
  );

  const isCurrent = useCallback((trackId: string) => current?.id === trackId, [current]);

  // Load the source whenever the current track changes, and autoplay if we were
  // already in a playing state (i.e. the user pressed play, then skipped).
  //
  // `playToken` is a dependency because the track id alone is not enough. When
  // "Phát tất cả" resolved to the track already loaded — the common case once a
  // queue has been started, and guaranteed after it ends — this effect did not
  // re-run, so nothing called `load()`/`play()`. The element stayed paused while
  // `playing` had already been set true, and since a paused element fires no
  // `pause` event there was nothing to correct it: the bar showed a Pause button
  // over silence, stuck at the end of the track. Every explicit play request now
  // bumps the token and restarts the source, unchanged id or not.
  useEffect(() => {
    const el = audioRef.current;
    if (!el || !current?.audio_asset_id) return;
    el.src = trackAudioURL(current.audio_asset_id);
    el.load();
    setPosition(0);
    setDuration(0);
    // Any parked scrub belonged to the track being replaced.
    pendingSeekRef.current = null;
    if (playing) play();
    // `playing` is intentionally omitted: this effect is about the SOURCE
    // changing. Play/pause on an unchanged source is handled by `toggle`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.id, current?.audio_asset_id, playToken, play]);

  // Wire the media element's events to state, once.
  useEffect(() => {
    const el = audioRef.current;
    if (!el) return;

    const onTime = () => setPosition(el.currentTime);
    const onMeta = () => {
      const known = Number.isFinite(el.duration) ? el.duration : 0;
      setDuration(known);
      // Apply a scrub that arrived while the duration was still unknown.
      const pending = pendingSeekRef.current;
      if (pending !== null && known > 0) {
        pendingSeekRef.current = null;
        el.currentTime = (pending / 100) * known;
        setPosition(el.currentTime);
      }
    };
    const onPlay = () => setPlaying(true);
    // A track reaching its end fires `pause` as well as `ended`, and the pause
    // lands FIRST. Treating that as "the user paused" set `playing` to false
    // before `ended` advanced the index, so the next track loaded and then sat
    // there: the queue stopped dead after every single song. `el.ended`
    // distinguishes the two — a real pause never has it set.
    const onPause = () => {
      if (!el.ended) setPlaying(false);
    };
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
      jumpTo,
    }),
    [
      queue, index, current, playing, position, duration, progressPct, shuffle, repeat, error,
      playQueue, playTrack, toggle, next, prev, seekPct, toggleShuffle, cycleRepeat, stop, isCurrent,
      jumpTo,
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
