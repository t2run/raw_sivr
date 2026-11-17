import { useEffect, useState } from "react";
import { api, errorText, OrphanReport, OrphanResult, size } from "./api";

export default function OrphanDialog({ root, onClose }: { root: string; onClose: (changed: boolean) => void }) {
  const [where, setWhere] = useState(root);
  const [report, setReport] = useState<OrphanReport | null>(null);
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [result, setResult] = useState<OrphanResult | null>(null);

  useEffect(() => {
    if (!where) return;
    let live = true;
    setReport(null);
    setChecked(new Set());
    setConfirm(false);
    setError("");
    api
      .FindOrphans(where)
      .then((r) => live && setReport(r))
      .catch((e) => live && setError(errorText(e)));
    return () => {
      live = false;
    };
  }, [where]);

  const chosen = report?.folders.filter((f) => checked.has(f.dir)) ?? [];
  const files = chosen.reduce((n, f) => n + f.files.length, 0);
  const bytes = chosen.reduce((n, f) => n + f.bytes, 0);

  const toggle = (dir: string) => {
    const next = new Set(checked);
    next.has(dir) ? next.delete(dir) : next.add(dir);
    setChecked(next);
    setConfirm(false);
  };

  const run = async () => {
    if (!confirm) {
      setConfirm(true);
      return;
    }
    setBusy(true);
    try {
      setResult(await api.RecycleOrphans(where, [...checked]));
    } catch (e) {
      setError(errorText(e));
    }
    setBusy(false);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="flex max-h-[80vh] w-[620px] flex-col rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl">
        <h2 className="text-lg font-semibold text-white">Find orphan RAW files</h2>
        <div className="mt-1 text-xs text-zinc-400">
          An orphan is a RAW file with no JPG of the same name anywhere in the folder below, including its subfolders.
          If you culled by deleting JPGs, these are the RAWs left behind.
        </div>

        <div className="mt-4 flex items-center gap-2">
          <div className="min-w-0 flex-1 truncate rounded-lg bg-zinc-950 px-3 py-2 text-sm text-zinc-300" title={where}>
            {where || "No folder chosen"}
          </div>
          <button
            disabled={busy}
            onClick={async () => {
              const p = await api.ChooseFolder();
              if (p) {
                setResult(null);
                setWhere(p);
              }
            }}
            className="shrink-0 rounded-lg bg-zinc-800 px-3 py-2 text-sm text-zinc-300 hover:bg-zinc-700"
          >
            Choose folder…
          </button>
        </div>

        {result ? (
          <div className="mt-4 rounded-xl bg-zinc-950 px-4 py-3 text-sm text-zinc-300">
            {result.recycled} files sent to the Recycle Bin, {size(result.bytes)} freed once you empty it.
            {result.failed.length > 0 && (
              <div className="mt-2 max-h-24 overflow-auto text-xs text-rose-400">
                {result.failed.map((f) => (
                  <div key={f}>{f}</div>
                ))}
              </div>
            )}
          </div>
        ) : (
          <>
            <div className="mt-3 text-sm text-zinc-300">
              {!report && !error && where && <span className="text-zinc-500">Searching…</span>}
              {report &&
                (report.files === 0
                  ? `No orphans: all ${report.raws} RAW files have a JPG.`
                  : `${report.files} of ${report.raws} RAW files are orphans (${size(report.bytes)}). Tick the folders to clean.`)}
            </div>
            {report && report.folders.length > 0 && (
              <div className="mt-2 min-h-0 flex-1 overflow-y-auto rounded-xl bg-zinc-950 p-1">
                {report.folders.map((f) => (
                  <label
                    key={f.dir}
                    title={f.files.map((p) => p.slice(f.dir.length + 1)).join(", ")}
                    className="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm hover:bg-zinc-900"
                  >
                    <input
                      type="checkbox"
                      checked={checked.has(f.dir)}
                      onChange={() => toggle(f.dir)}
                      className="accent-emerald-500"
                    />
                    <span className="min-w-0 flex-1 truncate text-zinc-200">{f.rel === "." ? "(this folder)" : f.rel}</span>
                    <span className="shrink-0 tabular-nums text-zinc-500">
                      {f.files.length} files · {size(f.bytes)}
                    </span>
                  </label>
                ))}
              </div>
            )}
          </>
        )}
        {error && <div className="mt-3 text-sm text-rose-400">{error}</div>}

        <div className="mt-5 flex items-center justify-end gap-2">
          {confirm && !result && (
            <span className="mr-auto text-xs text-amber-400">Click again to confirm. Files go to the Recycle Bin.</span>
          )}
          <button
            disabled={busy}
            onClick={() => onClose(result !== null)}
            className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800"
          >
            {result ? "Close" : "Cancel"}
          </button>
          {!result && (
            <button
              disabled={busy || files === 0}
              onClick={run}
              className="rounded-lg bg-rose-500 px-4 py-2 text-sm font-medium text-white hover:bg-rose-400 disabled:opacity-40"
            >
              {busy ? "Working…" : files === 0 ? "Recycle" : `Recycle ${files} files (${size(bytes)})`}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
