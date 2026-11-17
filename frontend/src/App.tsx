import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, ApplyResult, Decision, errorText, Folder, Item, Mode, on, PhotoInfo, ProxyState, size } from "./api";
import OrphanDialog from "./OrphanDialog";
import ApplyDialog from "./ApplyDialog";
import logo from "./assets/logo.png";
import ImportDialog from "./ImportDialog";
import SettingsDialog from "./SettingsDialog";
import VideoPlayer from "./VideoPlayer";
import ZoomImage, { Zoom } from "./ZoomImage";

type Filter = "all" | "unmarked" | "pick" | "reject";

// Shortcuts that can be changed in Settings; the saved ones override these.
const DEFAULT_KEYS: Record<string, string> = {
  pick: "p",
  reject: "x",
  clear: "u",
  zoom: "z",
  info: "i",
  compare: "c",
  burst: "b",
  keepBest: "k",
  original: "o",
};
type Toast = { text: string; undo?: boolean; error?: boolean };

const FILTERS: { key: Filter; label: string }[] = [
  { key: "all", label: "All" },
  { key: "unmarked", label: "Unmarked" },
  { key: "pick", label: "Picked" },
  { key: "reject", label: "Rejected" },
];

function matches(it: Item, f: Filter): boolean {
  return f === "all" || (f === "unmarked" ? it.decision === "" : it.decision === f);
}

function Badge({ item }: { item: Item }) {
  if (item.decision === "pick")
    return <span className="rounded-md bg-emerald-500 px-1.5 py-0.5 text-[11px] font-semibold text-black">PICK</span>;
  if (item.decision === "reject")
    return <span className="rounded-md bg-rose-500 px-1.5 py-0.5 text-[11px] font-semibold text-white">REJECT</span>;
  return null;
}

function fileKinds(item: Item): string {
  if (item.kind === "video") return "VIDEO";
  return [item.hasJpg && "JPG", item.hasRaw && "RAW"].filter(Boolean).join(" + ");
}

export default function App() {
  const [folder, setFolder] = useState<Folder | null>(null);
  const [filter, setFilter] = useState<Filter>("all");
  const [index, setIndex] = useState(0);
  const [single, setSingle] = useState(false);
  const [applying, setApplying] = useState(false);
  const [importing, setImporting] = useState(false);
  const [settings, setSettings] = useState(false);
  const [orphans, setOrphans] = useState(false);
  const [keys, setKeys] = useState<Record<string, string>>(DEFAULT_KEYS);
  const key = (action: string) => (keys[action] ?? "").toUpperCase();
  const loadKeys = useCallback(() => {
    api.GetSettings().then((s) => setKeys({ ...DEFAULT_KEYS, ...s.keys }));
  }, []);
  useEffect(loadKeys, [loadKeys]);
  const dialogOpen = applying || importing || settings || orphans;
  const [toast, setToast] = useState<Toast | null>(null);
  const [loading, setLoading] = useState(false);
  const grid = useRef<HTMLDivElement>(null);
  const strip = useRef<HTMLDivElement>(null);
  const togglePlay = useRef<() => void>(() => {});
  const [zoom, setZoom] = useState<Zoom>({ on: false, u: 0.5, v: 0.5 });
  const [compare, setCompare] = useState(1); // photos side by side in the single view
  const [showInfo, setShowInfo] = useState(true);
  const [info, setInfo] = useState<PhotoInfo | null>(null);
  const infoCache = useRef(new Map<string, PhotoInfo>());
  const compareStart = useRef(0);

  const items = folder?.items ?? [];
  const visible = useMemo(() => items.filter((it) => matches(it, filter)), [items, filter]);
  const sel = Math.min(index, Math.max(0, visible.length - 1));
  const current: Item | undefined = visible[sel];

  // The compare window slides only when the selection leaves it.
  let start = compareStart.current;
  if (sel < start) start = sel;
  if (sel >= start + compare) start = sel - compare + 1;
  start = Math.max(0, Math.min(start, visible.length - compare));
  compareStart.current = start;
  const panes = visible.slice(start, start + compare);

  useEffect(() => {
    setInfo(null);
    if (!single || !current || current.kind !== "photo") return;
    const id = current.id;
    const cached = infoCache.current.get(id);
    if (cached) {
      setInfo(cached);
      return;
    }
    let live = true;
    api
      .PhotoInfo(id)
      .then((i) => {
        infoCache.current.set(id, i);
        if (live) setInfo(i);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [single, current?.id, current?.kind]);

  const counts = useMemo(() => {
    const c = { all: items.length, unmarked: 0, pick: 0, reject: 0 };
    for (const it of items) c[it.decision === "" ? "unmarked" : it.decision]++;
    return c;
  }, [items]);

  const open = useCallback(async (root: string) => {
    setLoading(true);
    try {
      setFolder(await api.OpenFolder(root));
    } catch (e) {
      setToast({ text: errorText(e), error: true });
    } finally {
      setLoading(false);
    }
  }, []);

  const choose = useCallback(async () => {
    const root = await api.ChooseFolder();
    if (root) {
      setIndex(0);
      setSingle(false);
      setFilter("all");
      await open(root);
    }
  }, [open]);

  useEffect(() => {
    api.LastFolder().then((root) => {
      if (root) open(root);
    });
  }, [open]);

  useEffect(
    () =>
      on<{ video: string; proxy: ProxyState; proxyUrl?: string }>("proxy", (ev) =>
        setFolder(
          (f) =>
            f && {
              ...f,
              items: f.items.map((it) =>
                it.video === ev.video ? { ...it, proxy: ev.proxy, proxyUrl: ev.proxyUrl } : it,
              ),
            },
        ),
      ),
    [],
  );

  useEffect(() => {
    if (!toast || toast.undo) return;
    const t = setTimeout(() => setToast(null), 6000);
    return () => clearTimeout(t);
  }, [toast]);

  const mark = useCallback(
    (decision: Decision) => {
      if (!current) return;
      const id = current.id;
      setFolder((f) => f && { ...f, items: f.items.map((it) => (it.id === id ? { ...it, decision } : it)) });
      api.SetDecision(id, decision).catch((e) => setToast({ text: errorText(e), error: true }));
      // Move on to the next item, unless the filter just removed this one
      // from the list (then the next item slides into its place).
      const staysVisible = filter === "all" || (filter === "unmarked" ? decision === "" : decision === filter);
      if (single && compare === 1 && decision !== "" && staysVisible) setIndex(Math.min(sel + 1, visible.length - 1));
      else setIndex(sel);
    },
    [current, filter, single, compare, sel, visible.length],
  );

  // Pick the current photo and reject every other frame of its burst.
  const keepBest = useCallback(() => {
    if (!current || !current.burst) return;
    const changes: Record<string, Decision> = {};
    for (const it of items) if (it.burst === current.burst) changes[it.id] = it.id === current.id ? "pick" : "reject";
    setFolder(
      (f) => f && { ...f, items: f.items.map((it) => (it.id in changes ? { ...it, decision: changes[it.id] } : it)) },
    );
    api.SetDecisions(changes).catch((e) => setToast({ text: errorText(e), error: true }));
  }, [current, items]);

  // Put the whole burst (up to four frames) side by side.
  const compareBurst = useCallback(() => {
    if (!current || !current.burst) return;
    const first = visible.findIndex((it) => it.burst === current.burst);
    const count = visible.filter((it) => it.burst === current.burst).length;
    if (count < 2) return;
    compareStart.current = first;
    setCompare(Math.min(4, count));
    setSingle(true);
  }, [current, visible]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (dialogOpen || e.ctrlKey || e.altKey || e.metaKey) return;
      if (e.target instanceof HTMLInputElement) return;
      const cols = single
        ? 1
        : Math.max(1, grid.current ? getComputedStyle(grid.current).gridTemplateColumns.split(" ").length : 1);
      const go = (n: number) => setIndex(Math.max(0, Math.min(visible.length - 1, sel + n)));
      // A rebindable action wins; otherwise the fixed keys below apply.
      const pressed = e.key.toLowerCase();
      const action = Object.keys(keys).find((name) => keys[name] === pressed);
      switch (action ?? e.key) {
        case "ArrowRight":
          go(1);
          break;
        case "ArrowLeft":
          go(-1);
          break;
        case "ArrowDown":
          go(cols);
          break;
        case "ArrowUp":
          go(-cols);
          break;
        case "pick":
          mark("pick");
          break;
        case "reject":
          mark("reject");
          break;
        case "clear":
          mark("");
          break;
        case "zoom":
          if (single) setZoom((z) => ({ ...z, on: !z.on }));
          break;
        case "keepBest":
          keepBest();
          break;
        case "burst":
          compareBurst();
          break;
        case "info":
          setShowInfo((v) => !v);
          break;
        case "compare":
          if (single) setCompare((n) => (n === 1 ? 2 : 1));
          break;
        case "2":
        case "3":
        case "4":
          if (single) setCompare(Number(e.key));
          break;
        case "Enter":
          if (current) setSingle((s) => !s);
          break;
        case "Escape":
          setSingle(false);
          break;
        case " ":
          if (single && current?.kind === "video") togglePlay.current();
          else if (current) setSingle((s) => !s);
          break;
        default:
          return;
      }
      e.preventDefault();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [dialogOpen, single, visible.length, sel, current, mark, keepBest, compareBurst, keys]);

  // Keep the selected tile or filmstrip frame in view.
  useEffect(() => {
    const host = single ? strip.current : grid.current;
    host?.querySelector(`[data-i="${sel}"]`)?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [sel, single, visible.length]);

  // Warm the cache for the neighbours so stepping through feels instant.
  useEffect(() => {
    if (!single) return;
    for (const n of [1, 2, -1]) {
      const it = visible[sel + n];
      if (it?.kind === "photo") new Image().src = it.full;
    }
  }, [single, sel, visible]);

  const applied = async (mode: Mode, r: ApplyResult) => {
    setApplying(false);
    const verb = mode === "copy" ? "Copied" : mode === "move" ? "Moved and cleaned up" : "Recycled";
    const freed = r.freedBytes > 0 ? ` ${size(r.freedBytes)} went to the Recycle Bin.` : "";
    setToast(
      r.failed.length
        ? { text: `${r.ok} files done, ${r.failed.length} failed. First: ${r.failed[0]}`, error: true, undo: true }
        : { text: `${verb} ${r.ok} files.${freed}`, undo: true },
    );
    if (folder) await open(folder.root);
  };

  const undo = async () => {
    try {
      const r = await api.Undo();
      setToast({
        text: r.recycled.length
          ? `Undone. ${r.recycled.length} recycled files must be restored from the Recycle Bin.`
          : "Undone.",
      });
    } catch (e) {
      setToast({ text: errorText(e), error: true });
    }
    if (folder) await open(folder.root);
  };

  const dialogs = (
    <>
      {applying && <ApplyDialog onClose={() => setApplying(false)} onDone={applied} />}
      {importing && (
        <ImportDialog
          onClose={() => setImporting(false)}
          onOpen={(dest) => {
            setImporting(false);
            setIndex(0);
            setSingle(false);
            setFilter("all");
            open(dest);
          }}
        />
      )}
      {orphans && (
        <OrphanDialog
          root={folder?.root ?? ""}
          onClose={(changed) => {
            setOrphans(false);
            if (changed && folder) open(folder.root);
          }}
        />
      )}
      {settings && (
        <SettingsDialog
          onClose={(saved) => {
            setSettings(false);
            if (saved) loadKeys();
            if (saved && folder) open(folder.root);
          }}
        />
      )}
    </>
  );

  if (!folder) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-5">
        <img src={logo} className="h-20 w-20" draggable={false} />
        <div className="text-3xl font-semibold tracking-tight text-white">RAW SIVR</div>
        <div className="max-w-md text-center text-sm text-zinc-400">
          Open the folder you copied from the camera. JPG and RAW files with the same name are shown as one photo.
        </div>
        <button
          onClick={choose}
          disabled={loading}
          className="rounded-xl bg-emerald-500 px-6 py-3 text-sm font-medium text-black hover:bg-emerald-400"
        >
          {loading ? "Opening…" : "Open a shoot folder"}
        </button>
        <button
          onClick={() => setImporting(true)}
          className="rounded-xl bg-zinc-800 px-6 py-3 text-sm text-zinc-200 hover:bg-zinc-700"
        >
          Import from a card
        </button>
        {dialogs}
        {toast && <div className="text-sm text-rose-400">{toast.text}</div>}
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-14 shrink-0 items-center gap-4 border-b border-zinc-800/80 bg-zinc-950 px-4">
        <div className="flex shrink-0 items-center gap-2 text-[15px] font-semibold tracking-tight text-white">
          <img src={logo} className="h-6 w-6" draggable={false} />
          RAW SIVR
        </div>
        <button
          onClick={choose}
          title={folder.root}
          className="max-w-[340px] truncate rounded-lg bg-zinc-900 px-3 py-1.5 text-left text-sm text-zinc-300 hover:bg-zinc-800"
          style={{ direction: "rtl" }}
        >
          <bdi>{folder.root}</bdi>
        </button>
        <div className="flex rounded-lg bg-zinc-900 p-0.5 text-sm">
          {FILTERS.map((f) => (
            <button
              key={f.key}
              onClick={() => {
                setFilter(f.key);
                setIndex(0);
              }}
              className={`rounded-md px-3 py-1 ${
                filter === f.key ? "bg-zinc-700 text-white" : "text-zinc-400 hover:text-zinc-200"
              }`}
            >
              {f.label} <span className="tabular-nums text-zinc-500">{counts[f.key]}</span>
            </button>
          ))}
        </div>
        <div className="flex-1" />
        {single && (
          <button onClick={() => setSingle(false)} className="rounded-lg px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800">
            Back to grid
          </button>
        )}
        <button onClick={() => setImporting(true)} className="rounded-lg px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800">
          Import…
        </button>
        <button onClick={() => setOrphans(true)} className="rounded-lg px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800">
          Orphan RAWs…
        </button>
        <button onClick={() => setSettings(true)} className="rounded-lg px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-800">
          Settings
        </button>
        <button
          onClick={() => setApplying(true)}
          disabled={counts.pick + counts.reject === 0}
          className="rounded-lg bg-emerald-500 px-4 py-1.5 text-sm font-medium text-black hover:bg-emerald-400 disabled:opacity-40"
        >
          Apply…
        </button>
      </header>

      {visible.length === 0 ? (
        <div className="flex flex-1 items-center justify-center text-sm text-zinc-500">
          {items.length === 0 ? "No photos or videos found in this folder." : "Nothing matches this filter."}
        </div>
      ) : single && current ? (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="relative min-h-0 flex-1 p-4">
            {compare > 1 ? (
              <div className="flex h-full gap-2">
                {panes.map((it, k) => (
                  <div
                    key={it.id}
                    className={`flex min-w-0 flex-1 flex-col overflow-hidden rounded-xl border ${
                      start + k === sel ? "border-white" : "border-zinc-800"
                    }`}
                  >
                    <button
                      onClick={() => setIndex(start + k)}
                      className="flex h-8 shrink-0 items-center gap-2 bg-zinc-900 px-3 text-xs hover:bg-zinc-800"
                    >
                      <span className="font-medium text-white">{it.name}</span>
                      <Badge item={it} />
                    </button>
                    <div className="min-h-0 flex-1">
                      {it.kind === "video" ? (
                        <img src={it.thumb} className="h-full w-full object-contain" draggable={false} />
                      ) : (
                        <ZoomImage src={it.full} zoom={zoom} onZoom={setZoom} />
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : current.kind === "video" ? (
              <VideoPlayer item={current} hasMpv={folder.hasMpv} suspended={dialogOpen} originalKey={keys.original} toggleRef={togglePlay} />
            ) : (
              <ZoomImage src={current.full} zoom={zoom} onZoom={setZoom} />
            )}
            {compare === 1 && (
              <div className="pointer-events-none absolute left-7 top-7 flex items-center gap-2">
                <Badge item={current} />
              </div>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-3 whitespace-nowrap px-4 pb-2 text-sm">
            <span className="font-medium text-white">{current.name}</span>
            <span className="text-zinc-500">{fileKinds(current)}</span>
            <span className="tabular-nums text-zinc-500">
              {sel + 1} / {visible.length}
            </span>
            {current.burst > 0 && (
              <span className="shrink-0 rounded-md bg-sky-500/15 px-2 py-0.5 text-sky-300">
                Burst {current.burstPos} of {current.burstSize}
              </span>
            )}
            {showInfo && info && (
              <span className="min-w-0 truncate text-zinc-400">
                {[info.shutter, info.aperture, info.iso, info.focal, info.camera, info.lens].filter(Boolean).join(" · ")}
              </span>
            )}
            <div className="flex-1" />
            {current.burst > 0 && (
              <>
                <button onClick={compareBurst} className="shrink-0 rounded-lg bg-zinc-800 px-3 py-1 text-zinc-300 hover:bg-zinc-700">
                  Compare burst ({key("burst")})
                </button>
                <button onClick={keepBest} className="shrink-0 rounded-lg bg-sky-500/15 px-3 py-1 text-sky-300 hover:bg-sky-500/25">
                  Keep, reject rest ({key("keepBest")})
                </button>
              </>
            )}
            <button
              onClick={() => setZoom((z) => ({ ...z, on: !z.on }))}
              className={`rounded-lg px-3 py-1 ${zoom.on ? "bg-zinc-600 text-white" : "bg-zinc-800 text-zinc-300 hover:bg-zinc-700"}`}
            >
              100% ({key("zoom")})
            </button>
            <button
              onClick={() => setCompare((n) => (n === 1 ? 2 : 1))}
              className={`rounded-lg px-3 py-1 ${compare > 1 ? "bg-zinc-600 text-white" : "bg-zinc-800 text-zinc-300 hover:bg-zinc-700"}`}
            >
              Compare ({key("compare")})
            </button>
            <button onClick={() => mark("pick")} className="rounded-lg bg-emerald-500/15 px-3 py-1 text-emerald-400 hover:bg-emerald-500/25">
              Pick ({key("pick")})
            </button>
            <button onClick={() => mark("reject")} className="rounded-lg bg-rose-500/15 px-3 py-1 text-rose-400 hover:bg-rose-500/25">
              Reject ({key("reject")})
            </button>
            <button onClick={() => mark("")} className="rounded-lg bg-zinc-800 px-3 py-1 text-zinc-300 hover:bg-zinc-700">
              Clear ({key("clear")})
            </button>
          </div>
          <div ref={strip} className="flex h-[84px] shrink-0 gap-1.5 overflow-x-auto border-t border-zinc-800/80 bg-zinc-950 px-4 py-2">
            {visible.map((it, i) => (
              <button
                key={it.id}
                data-i={i}
                onClick={() => setIndex(i)}
                className={`relative h-full shrink-0 overflow-hidden rounded-md ${
                  i === sel ? "outline-2 outline-white" : "opacity-70 outline-none hover:opacity-100"
                }`}
                style={{ aspectRatio: "3 / 2" }}
              >
                <img src={it.thumb} loading="lazy" className={`h-full w-full object-cover ${it.decision === "reject" ? "opacity-40" : ""}`} />
                {it.decision && (
                  <span className={`absolute inset-x-0 bottom-0 h-1 ${it.decision === "pick" ? "bg-emerald-400" : "bg-rose-500"}`} />
                )}
              </button>
            ))}
          </div>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          <div ref={grid} className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(230px, 1fr))" }}>
            {visible.map((it, i) => (
              <button
                key={it.id}
                data-i={i}
                onClick={() => setIndex(i)}
                onDoubleClick={() => {
                  setIndex(i);
                  setSingle(true);
                }}
                className={`group relative overflow-hidden rounded-xl bg-zinc-900 text-left ${
                  i === sel ? "outline-2 outline-offset-2 outline-white" : "outline-none"
                } ${it.decision === "pick" ? "ring-2 ring-emerald-400" : it.decision === "reject" ? "ring-2 ring-rose-500/70" : ""}`}
                style={{ aspectRatio: "3 / 2", contentVisibility: "auto" }}
              >
                <img
                  src={it.thumb}
                  loading="lazy"
                  draggable={false}
                  className={`h-full w-full object-cover transition ${it.decision === "reject" ? "opacity-35 grayscale" : ""}`}
                />
                <div className="absolute left-2 top-2 flex gap-1.5">
                  <Badge item={it} />
                  {it.burst > 0 && (
                    <span className="rounded-md bg-sky-500/90 px-1.5 py-0.5 text-[11px] font-semibold text-black">
                      BURST {it.burstPos}/{it.burstSize}
                    </span>
                  )}
                </div>
                {it.kind === "video" && (
                  <div className="absolute right-2 top-2 rounded-md bg-black/70 px-1.5 py-0.5 text-[11px] text-white">
                    {it.proxy?.status === "ready"
                      ? "▶ Video"
                      : it.proxy?.status === "error"
                        ? "Preview failed"
                        : "Building preview…"}
                  </div>
                )}
                <div className="absolute inset-x-0 bottom-0 flex items-end justify-between bg-gradient-to-t from-black/80 to-transparent px-2.5 pb-1.5 pt-6 text-xs">
                  <span className="font-medium text-white">{it.name}</span>
                  <span className="text-zinc-300">{fileKinds(it)}</span>
                </div>
              </button>
            ))}
          </div>
        </div>
      )}

      <footer className="flex h-8 shrink-0 items-center gap-5 border-t border-zinc-800/80 bg-zinc-950 px-4 text-xs text-zinc-500">
        <span>← → ↑ ↓ move</span>
        <span>{key("pick")} pick</span>
        <span>{key("reject")} reject</span>
        <span>{key("clear")} clear</span>
        <span>Enter open / close</span>
        <span>{key("zoom")} 100% zoom</span>
        <span>{key("compare")} compare · 2 3 4 panes</span>
        <span>
          {key("burst")} compare burst · {key("keepBest")} keep best
        </span>
        <span>{key("info")} info</span>
        <span>Space play video</span>
        <span>{key("original")} original video</span>
        {folder.strays.length > 0 && (
          <span className="ml-auto text-amber-500/80" title={folder.strays.join("\n")}>
            {folder.strays.length} video sidecar file{folder.strays.length > 1 ? "s" : ""} without a clip
          </span>
        )}
      </footer>

      {toast && (
        <div
          className={`fixed bottom-12 left-1/2 z-40 flex -translate-x-1/2 items-center gap-4 rounded-xl border px-4 py-2.5 text-sm shadow-2xl ${
            toast.error ? "border-rose-800 bg-rose-950 text-rose-200" : "border-zinc-700 bg-zinc-900 text-zinc-100"
          }`}
        >
          <span>{toast.text}</span>
          {toast.undo && (
            <button onClick={undo} className="font-medium text-emerald-400 hover:text-emerald-300">
              Undo
            </button>
          )}
          <button onClick={() => setToast(null)} className="text-zinc-500 hover:text-zinc-300">
            Dismiss
          </button>
        </div>
      )}

      {dialogs}
    </div>
  );
}
