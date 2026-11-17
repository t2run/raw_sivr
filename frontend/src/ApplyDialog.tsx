import { useEffect, useState } from "react";
import { api, ApplyResult, errorText, Mode, on, Plan, size } from "./api";

const MODES: { mode: Mode; title: string; text: string }[] = [
  {
    mode: "copy",
    title: "Copy picks",
    text: "Copies picked files into the selected folders inside this shoot folder. Nothing else is touched.",
  },
  {
    mode: "move",
    title: "Move picks and clean up",
    text: "Moves picked files into the selected folders and sends rejects to the Recycle Bin. Unmarked files stay.",
  },
  {
    mode: "cleanup",
    title: "Clean up rejects only",
    text: "Sends rejected files to the Recycle Bin. Picks and unmarked files stay where they are.",
  },
];

export default function ApplyDialog({
  onClose,
  onDone,
}: {
  onClose: () => void;
  onDone: (mode: Mode, result: ApplyResult) => void;
}) {
  const [mode, setMode] = useState<Mode>("copy");
  const [plan, setPlan] = useState<Plan | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState({ done: 0, total: 0 });

  useEffect(() => {
    let current = true;
    setPlan(null);
    setError("");
    api
      .Plan(mode)
      .then((p) => current && setPlan(p))
      .catch((e) => current && setError(errorText(e)));
    return () => {
      current = false;
    };
  }, [mode]);

  useEffect(() => on<{ done: number; total: number }>("progress", setProgress), []);

  const ops = plan?.ops ?? [];
  const count = (a: string) => ops.filter((o) => o.action === a).length;
  const transfers = count("copy") + count("move");
  const recycles = count("recycle");
  const conflicts = plan?.conflicts ?? [];
  const already = plan?.done?.length ?? 0;

  const run = async () => {
    setBusy(true);
    setError("");
    try {
      onDone(mode, await api.Apply(mode));
    } catch (e) {
      setError(errorText(e));
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="w-[560px] rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl">
        <h2 className="text-lg font-semibold text-white">Apply your selection</h2>
        <div className="mt-4 flex flex-col gap-2">
          {MODES.map((m) => (
            <button
              key={m.mode}
              disabled={busy}
              onClick={() => setMode(m.mode)}
              className={`rounded-xl border px-4 py-3 text-left transition ${
                mode === m.mode ? "border-emerald-500 bg-emerald-500/10" : "border-zinc-800 hover:border-zinc-600"
              }`}
            >
              <div className="text-sm font-medium text-white">{m.title}</div>
              <div className="mt-0.5 text-xs text-zinc-400">{m.text}</div>
            </button>
          ))}
        </div>

        <div className="mt-4 min-h-[76px] rounded-xl bg-zinc-950 px-4 py-3 text-sm text-zinc-300">
          {!plan && !error && <span className="text-zinc-500">Checking the folder…</span>}
          {plan && (
            <>
              <div>
                {plan.picks} picked · {plan.rejects} rejected · {plan.unmarked} unmarked
              </div>
              <div className="mt-1 text-zinc-400">
                {transfers > 0 && (
                  <div>
                    {transfers} files to {mode === "move" ? "move" : "copy"} ({(plan.bytes / 1048576).toFixed(0)} MB)
                  </div>
                )}
                {recycles > 0 && (
                  <div>
                    {recycles} files to the Recycle Bin (frees {size(plan.recycleBytes)})
                  </div>
                )}
                {already > 0 && <div>{already} files are already in the selected folders</div>}
                {ops.length === 0 && <div>Nothing to do in this mode.</div>}
              </div>
              {conflicts.length > 0 && (
                <div className="mt-2 text-amber-400">
                  {conflicts.length} files will be skipped because a different file with the same name is already
                  there:
                  <div className="mt-1 max-h-16 overflow-auto text-xs text-amber-400/80">
                    {conflicts.map((c) => (
                      <div key={c}>{c}</div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
          {error && <div className="text-rose-400">{error}</div>}
        </div>

        {busy && progress.total > 0 && (
          <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-zinc-800">
            <div
              className="h-full bg-emerald-500 transition-all"
              style={{ width: `${(100 * (progress.done + 1)) / progress.total}%` }}
            />
          </div>
        )}

        <div className="mt-5 flex justify-end gap-2">
          <button
            disabled={busy}
            onClick={onClose}
            className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800 disabled:opacity-40"
          >
            Cancel
          </button>
          <button
            disabled={busy || ops.length === 0}
            onClick={run}
            className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-medium text-black hover:bg-emerald-400 disabled:opacity-40"
          >
            {busy ? "Working…" : `Apply to ${ops.length} files`}
          </button>
        </div>
      </div>
    </div>
  );
}
