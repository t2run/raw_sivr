// Thin typed layer over the Go methods and events that Wails exposes.

export type Decision = "pick" | "reject" | "";
export type Mode = "copy" | "move" | "cleanup";

export interface ProxyState {
  status: "queued" | "building" | "ready" | "error";
  err?: string;
}

export interface Item {
  id: string;
  kind: "photo" | "video";
  name: string;
  hasJpg: boolean;
  hasRaw: boolean;
  decision: Decision;
  thumb: string;
  full: string;
  video?: string;
  proxy?: ProxyState;
  proxyUrl?: string;
  burst: number; // burst number, 0 when the photo is on its own
  burstPos: number;
  burstSize: number;
}

export interface OrphanFolder {
  dir: string;
  rel: string;
  files: string[];
  bytes: number;
}

export interface OrphanReport {
  root: string;
  folders: OrphanFolder[];
  files: number;
  bytes: number;
  raws: number;
}

export interface OrphanResult {
  recycled: number;
  bytes: number;
  failed: string[];
}

export interface Folder {
  root: string;
  items: Item[];
  strays: string[];
  hasMpv: boolean;
}

export interface Op {
  action: "copy" | "move" | "recycle";
  itemId: string;
  src: string;
  dst?: string;
}

export interface Plan {
  mode: Mode;
  ops: Op[] | null;
  done: string[] | null;
  conflicts: string[] | null;
  picks: number;
  rejects: number;
  unmarked: number;
  bytes: number;
  recycleBytes: number;
}

export interface ApplyResult {
  ok: number;
  failed: string[];
  freedBytes: number;
}

export interface UndoResult {
  mode: string;
  recycled: string[];
}

export interface PhotoInfo {
  camera: string;
  lens: string;
  shutter: string;
  aperture: string;
  iso: string;
  focal: string;
  taken: string;
}

export interface Layout {
  jpg: string;
  raw: string;
  video: string;
}

export interface Settings {
  LastFolder: string;
  library: string;
  folders: Layout;
  keys: Record<string, string>;
}

export interface ImportSource {
  path: string;
  label: string;
  photos: number;
  videos: number;
  files: number;
  bytes: number;
  date: string;
}

export interface ImportProgress {
  done: number;
  total: number;
  bytesDone: number;
  bytesTotal: number;
  file: string;
}

export interface ImportResult {
  dest: string;
  copied: number;
  skipped: number;
  failed: string[];
  cancelled: boolean;
}

export function size(bytes: number): string {
  if (bytes < 1 << 20) return `${Math.round(bytes / 1024)} KB`;
  return bytes >= 1 << 30 ? `${(bytes / (1 << 30)).toFixed(1)} GB` : `${Math.round(bytes / (1 << 20))} MB`;
}

interface Backend {
  SetDecisions(decisions: Record<string, Decision>): Promise<void>;
  FindOrphans(root: string): Promise<OrphanReport>;
  RecycleOrphans(root: string, dirs: string[]): Promise<OrphanResult>;
  GetSettings(): Promise<Settings>;
  SaveSettings(library: string, folders: Layout, keys: Record<string, string>): Promise<void>;
  ImportSources(): Promise<ImportSource[]>;
  InspectSource(path: string): Promise<ImportSource>;
  Import(source: string, name: string): Promise<ImportResult>;
  CancelImport(): Promise<void>;
  PhotoInfo(id: string): Promise<PhotoInfo>;
  LastFolder(): Promise<string>;
  ChooseFolder(): Promise<string>;
  OpenFolder(root: string): Promise<Folder>;
  SetDecision(id: string, decision: Decision): Promise<void>;
  Plan(mode: Mode): Promise<Plan>;
  Apply(mode: Mode): Promise<ApplyResult>;
  Undo(): Promise<UndoResult>;
  MpvPlay(path: string): Promise<void>;
  MpvRect(x: number, y: number, w: number, h: number): Promise<void>;
  MpvHide(): Promise<void>;
  MpvPause(paused: boolean): Promise<void>;
  MpvSeek(seconds: number): Promise<void>;
}

declare global {
  interface Window {
    go: { main: { App: Backend } };
    runtime: {
      EventsOn(name: string, cb: (data: any) => void): () => void;
    };
  }
}

export const api: Backend = new Proxy({} as Backend, {
  get:
    (_, method: string) =>
    (...args: unknown[]) =>
      (window.go.main.App as any)[method](...args),
});

export function on<T>(event: string, cb: (data: T) => void): () => void {
  return window.runtime.EventsOn(event, cb);
}

export function errorText(e: unknown): string {
  return typeof e === "string" ? e : e instanceof Error ? e.message : String(e);
}
