export namespace exif {
	
	export class Info {
	    camera: string;
	    lens: string;
	    shutter: string;
	    aperture: string;
	    iso: string;
	    focal: string;
	    taken: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.camera = source["camera"];
	        this.lens = source["lens"];
	        this.shutter = source["shutter"];
	        this.aperture = source["aperture"];
	        this.iso = source["iso"];
	        this.focal = source["focal"];
	        this.taken = source["taken"];
	    }
	}

}

export namespace importer {
	
	export class Result {
	    dest: string;
	    copied: number;
	    skipped: number;
	    failed: string[];
	    cancelled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dest = source["dest"];
	        this.copied = source["copied"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.cancelled = source["cancelled"];
	    }
	}
	export class Source {
	    path: string;
	    label: string;
	    photos: number;
	    videos: number;
	    files: number;
	    bytes: number;
	    date: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.photos = source["photos"];
	        this.videos = source["videos"];
	        this.files = source["files"];
	        this.bytes = source["bytes"];
	        this.date = source["date"];
	    }
	}

}

export namespace main {
	
	export class ApplyResult {
	    ok: number;
	    failed: string[];
	    freedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.failed = source["failed"];
	        this.freedBytes = source["freedBytes"];
	    }
	}
	export class Item {
	    burst: number;
	    burstPos: number;
	    burstSize: number;
	    id: string;
	    kind: string;
	    name: string;
	    hasJpg: boolean;
	    hasRaw: boolean;
	    decision: string;
	    thumb: string;
	    full: string;
	    video?: string;
	    proxy?: video.ProxyState;
	    proxyUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.burst = source["burst"];
	        this.burstPos = source["burstPos"];
	        this.burstSize = source["burstSize"];
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.hasJpg = source["hasJpg"];
	        this.hasRaw = source["hasRaw"];
	        this.decision = source["decision"];
	        this.thumb = source["thumb"];
	        this.full = source["full"];
	        this.video = source["video"];
	        this.proxy = this.convertValues(source["proxy"], video.ProxyState);
	        this.proxyUrl = source["proxyUrl"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Folder {
	    root: string;
	    items: Item[];
	    strays: string[];
	    hasMpv: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.items = this.convertValues(source["items"], Item);
	        this.strays = source["strays"];
	        this.hasMpv = source["hasMpv"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class OrphanResult {
	    recycled: number;
	    bytes: number;
	    failed: string[];
	
	    static createFrom(source: any = {}) {
	        return new OrphanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recycled = source["recycled"];
	        this.bytes = source["bytes"];
	        this.failed = source["failed"];
	    }
	}
	export class Settings {
	    LastFolder: string;
	    library: string;
	    folders: scan.Layout;
	    keys: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.LastFolder = source["LastFolder"];
	        this.library = source["library"];
	        this.folders = this.convertValues(source["folders"], scan.Layout);
	        this.keys = source["keys"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UndoResult {
	    mode: string;
	    recycled: string[];
	
	    static createFrom(source: any = {}) {
	        return new UndoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.recycled = source["recycled"];
	    }
	}

}

export namespace orphans {
	
	export class Folder {
	    dir: string;
	    rel: string;
	    files: string[];
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.rel = source["rel"];
	        this.files = source["files"];
	        this.bytes = source["bytes"];
	    }
	}
	export class Report {
	    root: string;
	    folders: Folder[];
	    files: number;
	    bytes: number;
	    raws: number;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.folders = this.convertValues(source["folders"], Folder);
	        this.files = source["files"];
	        this.bytes = source["bytes"];
	        this.raws = source["raws"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace plan {
	
	export class Op {
	    action: string;
	    itemId: string;
	    src: string;
	    dst?: string;
	
	    static createFrom(source: any = {}) {
	        return new Op(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.itemId = source["itemId"];
	        this.src = source["src"];
	        this.dst = source["dst"];
	    }
	}
	export class Plan {
	    mode: string;
	    ops: Op[];
	    done?: string[];
	    conflicts?: string[];
	    picks: number;
	    rejects: number;
	    unmarked: number;
	    bytes: number;
	    recycleBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.ops = this.convertValues(source["ops"], Op);
	        this.done = source["done"];
	        this.conflicts = source["conflicts"];
	        this.picks = source["picks"];
	        this.rejects = source["rejects"];
	        this.unmarked = source["unmarked"];
	        this.bytes = source["bytes"];
	        this.recycleBytes = source["recycleBytes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace scan {
	
	export class Layout {
	    jpg: string;
	    raw: string;
	    video: string;
	
	    static createFrom(source: any = {}) {
	        return new Layout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jpg = source["jpg"];
	        this.raw = source["raw"];
	        this.video = source["video"];
	    }
	}

}

export namespace video {
	
	export class ProxyState {
	    status: string;
	    path?: string;
	    err?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.path = source["path"];
	        this.err = source["err"];
	    }
	}

}

