import { useEffect, useState } from "react";
import { api, errorText, Layout } from "./api";

const ACTIONS: { action: string; label: string }[] = [
  { action: "pick", label: "Pick" },
  { action: "reject", label: "Reject" },
  { action: "clear", label: "Clear mark" },
  { action: "zoom", label: "100% zoom" },
  { action: "compare", label: "Compare" },
  { action: "burst", label: "Compare burst" },
  { action: "keepBest", label: "Keep best of burst" },
  { action: "info", label: "Show / hide info" },
  { action: "original", label: "Original video" },
];

const FIELDS: { key: keyof Layout; label: string }[] = [
  { key: "jpg", label: "Picked JPGs go to" },
  { key: "raw", label: "Picked RAWs go to" },
  { key: "video", label: "Picked videos go to" },
];

export default function SettingsDialog({ onClose }: { onClose: (saved: boolean) => void }) {
  const [library, setLibrary] = useState("");
  const [folders, setFolders] = useState<Layout | null>(null);
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    api.GetSettings().then((s) => {
      setLibrary(s.library);
      setFolders(s.folders);
      setKeys(s.keys);
    });
  }, []);

  const save = async () => {
    if (!folders) return;
    try {
      await api.SaveSettings(library, folders, keys);
      onClose(true);
    } catch (e) {
      setError(errorText(e));
    }
  };

  const input =
    "mt-1 w-full select-text rounded-lg border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-white outline-none focus:border-emerald-500";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="w-[560px] rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl">
        <h2 className="text-lg font-semibold text-white">Settings</h2>

        <label className="mt-4 block text-xs text-zinc-400">
          Library folder (imports create a dated folder in here)
          <div className="flex gap-2">
            <input value={library} onChange={(e) => setLibrary(e.target.value)} className={input} />
            <button
              onClick={async () => {
                const p = await api.ChooseFolder();
                if (p) setLibrary(p);
              }}
              className="mt-1 shrink-0 rounded-lg bg-zinc-800 px-3 text-sm text-zinc-300 hover:bg-zinc-700"
            >
              Browse…
            </button>
          </div>
        </label>

        {folders &&
          FIELDS.map((f) => (
            <label key={f.key} className="mt-3 block text-xs text-zinc-400">
              {f.label}
              <input
                value={folders[f.key]}
                onChange={(e) => setFolders({ ...folders, [f.key]: e.target.value })}
                className={input}
              />
            </label>
          ))}
        <div className="mt-2 text-xs text-zinc-500">
          These folders are created inside the shoot folder. Files already in the old folders are not moved.
        </div>

        <div className="mt-4 text-xs text-zinc-400">Keyboard shortcuts (one letter each)</div>
        <div className="mt-2 grid grid-cols-3 gap-x-4 gap-y-2">
          {ACTIONS.map((a) => (
            <label key={a.action} className="flex items-center justify-between gap-2 text-xs text-zinc-300">
              {a.label}
              <input
                value={(keys[a.action] ?? "").toUpperCase()}
                maxLength={1}
                onFocus={(e) => e.target.select()}
                onChange={(e) => setKeys({ ...keys, [a.action]: e.target.value.toLowerCase() })}
                className="w-9 select-text rounded-md border border-zinc-700 bg-zinc-950 py-1 text-center text-sm text-white outline-none focus:border-emerald-500"
              />
            </label>
          ))}
        </div>
        <div className="mt-2 text-xs text-zinc-500">Arrows, Enter, Esc, Space and 2 / 3 / 4 are fixed.</div>
        {error && <div className="mt-3 text-sm text-rose-400">{error}</div>}

        <div className="mt-5 flex justify-end gap-2">
          <button onClick={() => onClose(false)} className="rounded-lg px-4 py-2 text-sm text-zinc-300 hover:bg-zinc-800">
            Cancel
          </button>
          <button
            onClick={save}
            className="rounded-lg bg-emerald-500 px-4 py-2 text-sm font-medium text-black hover:bg-emerald-400"
          >
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
