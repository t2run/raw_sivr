import { useEffect, useRef, useState } from "react";
import { api, errorText, Item, on } from "./api";

function clock(s: number): string {
  if (!isFinite(s) || s < 0) s = 0;
  const m = Math.floor(s / 60);
  return `${m}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
}

/**
 * Plays a clip two ways: the 1080p proxy in a normal <video> element, or the
 * camera original in mpv, which draws into a native window placed exactly
 * over the placeholder div. Nothing in the page can be drawn on top of that
 * native window, so its controls sit underneath it.
 */
export default function VideoPlayer({
  item,
  hasMpv,
  suspended,
  originalKey,
  toggleRef,
}: {
  originalKey: string; // shortcut that switches between preview and original
  item: Item;
  hasMpv: boolean;
  suspended: boolean; // a dialog is open: the native window must go away
  toggleRef: React.MutableRefObject<() => void>;
}) {
  const [original, setOriginal] = useState(false);
  const [error, setError] = useState("");
  const [pos, setPos] = useState(0);
  const [duration, setDuration] = useState(0);
  const [paused, setPaused] = useState(false);
  const [drops, setDrops] = useState(0);
  const [hwdec, setHwdec] = useState("");
  const holder = useRef<HTMLDivElement>(null);
  const videoEl = useRef<HTMLVideoElement>(null);
  const showOriginal = original && !suspended;

  useEffect(() => {
    setError("");
    setPos(0);
    setDrops(0);
  }, [item.id]);

  useEffect(() => {
    if (!showOriginal || !item.video) return;
    let alive = true;
    const place = () => {
      const el = holder.current;
      if (!el || !alive) return;
      const r = el.getBoundingClientRect();
      const k = window.devicePixelRatio;
      api.MpvRect(Math.round(r.left * k), Math.round(r.top * k), Math.round(r.width * k), Math.round(r.height * k));
    };
    api
      .MpvPlay(item.video)
      .then(place)
      .catch((e) => {
        setError(errorText(e));
        setOriginal(false);
      });
    const ro = new ResizeObserver(place);
    if (holder.current) ro.observe(holder.current);
    window.addEventListener("resize", place);
    const off = on<{ name: string; value: any }>("mpv", (ev) => {
      if (ev.name === "time-pos" && typeof ev.value === "number") setPos(ev.value);
      else if (ev.name === "duration" && typeof ev.value === "number") setDuration(ev.value);
      else if (ev.name === "pause") setPaused(!!ev.value);
      else if (ev.name === "frame-drop-count" && typeof ev.value === "number") setDrops(ev.value);
      else if (ev.name === "hwdec-current") setHwdec(typeof ev.value === "string" ? ev.value : "");
    });
    return () => {
      alive = false;
      ro.disconnect();
      window.removeEventListener("resize", place);
      off();
      api.MpvHide();
    };
  }, [showOriginal, item.video]);

  // A shortcut (O by default) switches between the preview and the original.
  useEffect(() => {
    if (!hasMpv || suspended) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() === originalKey && !e.ctrlKey && !e.altKey && !e.metaKey) setOriginal((v) => !v);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [hasMpv, suspended, originalKey]);

  toggleRef.current = () => {
    if (showOriginal) api.MpvPause(!paused);
    else if (videoEl.current) videoEl.current.paused ? videoEl.current.play() : videoEl.current.pause();
  };

  const status = item.proxy?.status;
  return (
    <div className="flex h-full w-full flex-col gap-3">
      <div className="relative min-h-0 flex-1 overflow-hidden rounded-xl bg-black">
        {showOriginal ? (
          <div ref={holder} className="h-full w-full" />
        ) : item.proxyUrl ? (
          <video
            ref={videoEl}
            key={item.proxyUrl}
            src={item.proxyUrl}
            poster={item.thumb}
            controls
            autoPlay
            className="h-full w-full object-contain"
          />
        ) : (
          <>
            <img src={item.thumb} className="h-full w-full object-contain opacity-50" />
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="rounded-lg bg-zinc-900/90 px-4 py-3 text-sm text-zinc-300">
                {status === "error"
                  ? `Preview failed: ${item.proxy?.err ?? "unknown error"}`
                  : status === "building"
                    ? "Building the preview for this clip…"
                    : "Preview is queued behind other clips…"}
              </div>
            </div>
          </>
        )}
      </div>

      <div className="flex h-9 items-center gap-3 text-sm">
        <div className="flex rounded-lg bg-zinc-900 p-0.5">
          <button
            onClick={() => setOriginal(false)}
            className={`rounded-md px-3 py-1 ${!original ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-zinc-200"}`}
          >
            Preview (1080p)
          </button>
          <button
            disabled={!hasMpv}
            title={hasMpv ? "Play the camera file with mpv" : "Install mpv to play originals"}
            onClick={() => setOriginal(true)}
            className={`rounded-md px-3 py-1 disabled:opacity-40 ${original ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-zinc-200"}`}
          >
            Original
          </button>
        </div>
        {showOriginal && (
          <>
            <button
              onClick={() => api.MpvPause(!paused)}
              className="w-16 rounded-md bg-zinc-800 px-3 py-1 hover:bg-zinc-700"
            >
              {paused ? "Play" : "Pause"}
            </button>
            <input
              type="range"
              min={0}
              max={duration || 1}
              step={0.05}
              value={Math.min(pos, duration || 1)}
              onChange={(e) => {
                const v = Number(e.target.value);
                setPos(v);
                api.MpvSeek(v);
              }}
              className="min-w-0 flex-1 accent-emerald-400"
            />
            <span className="tabular-nums text-zinc-400">
              {clock(pos)} / {clock(duration)}
            </span>
            <span className={`tabular-nums ${drops > 0 ? "text-amber-400" : "text-zinc-500"}`}>
              {drops} dropped · {hwdec && hwdec !== "no" ? `GPU (${hwdec})` : "CPU decode"}
            </span>
          </>
        )}
        {error && <span className="text-rose-400">{error}</span>}
      </div>
    </div>
  );
}
