import { useEffect, useState } from "react";
import { api, errorText, ImportProgress, ImportResult, ImportSource, on, size } from "./api";

export default function ImportDialog({ onClose, onOpen }: { onClose: () => void; onOpen: (folder: string) => void }) {
  const [sources, setSources] = useState<ImportSource[] | null>(null);
  const [source, setSource] = useState<ImportSource | null>(null);
  const [library, setLibrary] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<ImportProgress | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);

  useEffect(() => {
    api.GetSettings().then((s) => setLibrary(s.library));
    api
      .ImportSources()
      .then((list) => {
        setSources(list);
        setSource((cur) => cur ?? list[0] ?? null);
      })
      .catch((e) => setError(errorText(e)));
    return on<ImportProgress>("import", setProgress);
  }, []);

  const browse = async () => {
    const path = await api.ChooseFolder();
    if (!path) return;
    setError("");
    try {
      const s = await api.InspectSource(path);
      if (s.files === 0) setError("No photos or videos found in that folder.");
      else {
        setSources((list) => [...(list ?? []).filter((x) => x.path !== s.path), s]);
        setSource(s);
      }
    } catch (e) {
      setError(errorText(e));
    }
  };

  const run = async () => {
    if (!source) return;
    setBusy(true);
    setError("");
    setProgress(null);
    try {
      setResult(await api.Import(source.path, name));
    } catch (e) {
      setError(errorText(e));
    }
    setBusy(false);
  };

  const folder = source ? source.date + (name.trim() ? "_" + name.trim().replace(/ /g, "_") : "") : "";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="w-[580px] rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl">
        <h2 className="text-lg font-semibold text-white">Import from a card or folder</h2>

        {result ? (
          <>
            <div className="mt-4 rounded-xl bg-zinc-950 px-4 py-3 text-sm text-zinc-300">
              <div>
                {result.cancelled ? "Import stopped. " : ""}
                {result.copied} files copied
                {result.skipped > 0 && `, ${result.skipped} were already there`}
                {result.failed.length > 0 && `, ${result.failed.length} failed`}.
              </div>
              <div className="mt-1 break-all text-zinc-500">{result.dest}</div>
              {result.failed.length > 0 && (
                <div className="mt-2 max-h-24 overflow-auto text-xs text-rose-400">
                  {result.failed.map((f) => (
                    <div key={f}>{f}</div>
                  ))}
                </div>
              )}
              <div className="mt-2 text-zinc-500">Nothing was deleted from the source.</div>
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button onClick={onClose} className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800">
                Close
              </button>
              <button
                onClick={() => onOpen(result.dest)}
                className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-medium text-black hover:bg-emerald-400"
              >
                Open the folder
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="mt-4 flex flex-col gap-2">
              {sources === null && <div className="text-sm text-zinc-500">Looking for camera cards…</div>}
              {sources?.length === 0 && (
                <div className="text-sm text-zinc-500">No camera card found. Plug one in or choose a folder.</div>
              )}
              {sources?.map((s) => (
                <button
                  key={s.path}
                  disabled={busy}
                  onClick={() => setSource(s)}
                  className={`rounded-xl border px-4 py-3 text-left ${
                    source?.path === s.path ? "border-emerald-500 bg-emerald-500/10" : "border-zinc-800 hover:border-zinc-600"
                  }`}
                >
                  <div className="truncate text-sm font-medium text-white">{s.label}</div>
                  <div className="mt-0.5 text-xs text-zinc-400">
                    {s.photos} photos · {s.videos} videos · {size(s.bytes)} · shot {s.date}
                  </div>
                </button>
              ))}
              <button
                disabled={busy}
                onClick={browse}
                className="self-start rounded-lg bg-zinc-800 px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-700"
              >
                Choose a folder…
              </button>
            </div>

            <label className="mt-4 block text-xs text-zinc-400">
              Name of the shoot (optional)
              <input
                value={name}
                disabled={busy}
                onChange={(e) => setName(e.target.value)}
                placeholder="for example: Moon night"
                className="mt-1 w-full select-text rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-white outline-none focus:border-emerald-500"
              />
            </label>
            {source && (
              <div className="mt-2 break-all text-xs text-zinc-500">
                Copies to {library}\{folder}
              </div>
            )}
            {error && <div className="mt-3 text-sm text-rose-400">{error}</div>}

            {busy && progress && (
              <div className="mt-4">
                <div className="h-1.5 overflow-hidden rounded-full bg-zinc-800">
                  <div
                    className="h-full bg-emerald-500 transition-all"
                    style={{ width: `${(100 * progress.bytesDone) / Math.max(1, progress.bytesTotal)}%` }}
                  />
                </div>
                <div className="mt-1 text-xs text-zinc-500">
                  {progress.done} of {progress.total} · {size(progress.bytesDone)} of {size(progress.bytesTotal)} ·{" "}
                  {progress.file}
                </div>
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                onClick={busy ? () => api.CancelImport() : onClose}
                className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800"
              >
                {busy ? "Stop" : "Cancel"}
              </button>
              <button
                disabled={busy || !source}
                onClick={run}
                className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-medium text-black hover:bg-emerald-400 disabled:opacity-40"
              >
                {busy ? "Importing…" : source ? `Import ${source.files} files` : "Import"}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
