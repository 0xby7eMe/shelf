export namespace applog {
	
	export class Line {
	    time: number;
	    source: string;
	    app?: string;
	    level: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Line(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.source = source["source"];
	        this.app = source["app"];
	        this.level = source["level"];
	        this.text = source["text"];
	    }
	}

}

export namespace desktop {
	
	export class Settings {
	    menuEntries: boolean;
	    urlHandler: boolean;
	    tray: boolean;
	    closeToTray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.menuEntries = source["menuEntries"];
	        this.urlHandler = source["urlHandler"];
	        this.tray = source["tray"];
	        this.closeToTray = source["closeToTray"];
	    }
	}

}

export namespace epic {
	
	export class Account {
	    legendaryFound: boolean;
	    loggedIn: boolean;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.legendaryFound = source["legendaryFound"];
	        this.loggedIn = source["loggedIn"];
	        this.name = source["name"];
	    }
	}
	export class BattlEyeRuntime {
	    installed: boolean;
	    source: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new BattlEyeRuntime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.source = source["source"];
	        this.path = source["path"];
	    }
	}
	export class GameSettings {
	    protonPath: string;
	    launchArgs: string;
	    env: string;
	    mangoHud: boolean;
	    gameMode: boolean;
	    offline: boolean;
	    skipUpdateCheck: boolean;
	    cloudSaves: string;
	    autoUpdate: string;
	    savePath: string;
	
	    static createFrom(source: any = {}) {
	        return new GameSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protonPath = source["protonPath"];
	        this.launchArgs = source["launchArgs"];
	        this.env = source["env"];
	        this.mangoHud = source["mangoHud"];
	        this.gameMode = source["gameMode"];
	        this.offline = source["offline"];
	        this.skipUpdateCheck = source["skipUpdateCheck"];
	        this.cloudSaves = source["cloudSaves"];
	        this.autoUpdate = source["autoUpdate"];
	        this.savePath = source["savePath"];
	    }
	}
	export class Importable {
	    appName: string;
	    title: string;
	    path: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Importable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.source = source["source"];
	    }
	}
	export class LaunchInfo {
	    proton: string;
	    prefix: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proton = source["proton"];
	        this.prefix = source["prefix"];
	    }
	}
	export class PrefixUsage {
	    appName: string;
	    title: string;
	    path: string;
	    bytes: number;
	    installed: boolean;
	    shared?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrefixUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.title = source["title"];
	        this.path = source["path"];
	        this.bytes = source["bytes"];
	        this.installed = source["installed"];
	        this.shared = source["shared"];
	    }
	}
	export class Progress {
	    appName: string;
	    kind: string;
	    state: string;
	    percent: number;
	    eta?: string;
	    speed?: string;
	    error?: string;
	    indeterminate?: boolean;
	    position?: number;
	    damaged?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.percent = source["percent"];
	        this.eta = source["eta"];
	        this.speed = source["speed"];
	        this.error = source["error"];
	        this.indeterminate = source["indeterminate"];
	        this.position = source["position"];
	        this.damaged = source["damaged"];
	    }
	}
	export class ProtonBuild {
	    name: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtonBuild(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	    }
	}
	export class ProtonInstallState {
	    state: string;
	    percent: number;
	    message: string;
	    name?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtonInstallState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.percent = source["percent"];
	        this.message = source["message"];
	        this.name = source["name"];
	        this.error = source["error"];
	    }
	}
	export class QueueState {
	    jobs: Progress[];
	    paused: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueueState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobs = this.convertValues(source["jobs"], Progress);
	        this.paused = source["paused"];
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
	export class Settings {
	    installDir: string;
	    protonPath: string;
	    autoCheckUpdates: boolean;
	    autoUpdate: boolean;
	    cloudSaves: boolean;
	    ubisoftSoftwareRendering: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installDir = source["installDir"];
	        this.protonPath = source["protonPath"];
	        this.autoCheckUpdates = source["autoCheckUpdates"];
	        this.autoUpdate = source["autoUpdate"];
	        this.cloudSaves = source["cloudSaves"];
	        this.ubisoftSoftwareRendering = source["ubisoftSoftwareRendering"];
	    }
	}
	export class Tools {
	    mangoHud: boolean;
	    gameMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tools(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mangoHud = source["mangoHud"];
	        this.gameMode = source["gameMode"];
	    }
	}
	export class UbisoftInstalling {
	    key: string;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new UbisoftInstalling(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.bytes = source["bytes"];
	    }
	}
	export class UbisoftSetupState {
	    state: string;
	    percent: number;
	    message: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UbisoftSetupState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.percent = source["percent"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class UbisoftStatus {
	    connectInstalled: boolean;
	    proton: string;
	    protonIsGE: boolean;
	    running: boolean;
	    signedIn: boolean;
	    account: string;
	    games: number;
	
	    static createFrom(source: any = {}) {
	        return new UbisoftStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectInstalled = source["connectInstalled"];
	        this.proton = source["proton"];
	        this.protonIsGE = source["protonIsGE"];
	        this.running = source["running"];
	        this.signedIn = source["signedIn"];
	        this.account = source["account"];
	        this.games = source["games"];
	    }
	}
	export class UpdateInfo {
	    appName: string;
	    title: string;
	    installed: string;
	    latest: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.title = source["title"];
	        this.installed = source["installed"];
	        this.latest = source["latest"];
	    }
	}

}

export namespace library {
	
	export class Collection {
	    id: string;
	    name: string;
	    games: string[];
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.games = source["games"];
	    }
	}
	export class DayStat {
	    date: string;
	    minutes: number;
	
	    static createFrom(source: any = {}) {
	        return new DayStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.minutes = source["minutes"];
	    }
	}
	export class Game {
	    id: string;
	    source: string;
	    externalId: string;
	    name: string;
	    installed: boolean;
	    playtimeMinutes: number;
	    lastPlayed: number;
	    installPath?: string;
	    cover?: string;
	    sizeBytes?: number;
	    thirdParty?: string;
	    version?: string;
	    updateAvailable?: boolean;
	    antiCheat?: string;
	    cloudSaves?: boolean;
	    hero?: string;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.externalId = source["externalId"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.playtimeMinutes = source["playtimeMinutes"];
	        this.lastPlayed = source["lastPlayed"];
	        this.installPath = source["installPath"];
	        this.cover = source["cover"];
	        this.sizeBytes = source["sizeBytes"];
	        this.thirdParty = source["thirdParty"];
	        this.version = source["version"];
	        this.updateAvailable = source["updateAvailable"];
	        this.antiCheat = source["antiCheat"];
	        this.cloudSaves = source["cloudSaves"];
	        this.hero = source["hero"];
	    }
	}
	export class GameStat {
	    appId: string;
	    weekMinutes: number;
	    trackedMinutes: number;
	    sessions: number;
	
	    static createFrom(source: any = {}) {
	        return new GameStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.weekMinutes = source["weekMinutes"];
	        this.trackedMinutes = source["trackedMinutes"];
	        this.sessions = source["sessions"];
	    }
	}
	export class Organization {
	    collections: Collection[];
	    tags: Record<string, Array<string>>;
	
	    static createFrom(source: any = {}) {
	        return new Organization(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.collections = this.convertValues(source["collections"], Collection);
	        this.tags = source["tags"];
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
	export class Session {
	    appId: string;
	    since: number;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.since = source["since"];
	    }
	}
	export class Stats {
	    days: DayStat[];
	    weekMinutes: number;
	    prevWeekMinutes: number;
	    trackedMinutes: number;
	    sessions: number;
	    since: number;
	    games: GameStat[];
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.days = this.convertValues(source["days"], DayStat);
	        this.weekMinutes = source["weekMinutes"];
	        this.prevWeekMinutes = source["prevWeekMinutes"];
	        this.trackedMinutes = source["trackedMinutes"];
	        this.sessions = source["sessions"];
	        this.since = source["since"];
	        this.games = this.convertValues(source["games"], GameStat);
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
	export class Volume {
	    path: string;
	    total: number;
	    free: number;
	
	    static createFrom(source: any = {}) {
	        return new Volume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.total = source["total"];
	        this.free = source["free"];
	    }
	}

}

export namespace main {
	
	export class DesktopStatus {
	    settings: desktop.Settings;
	    trayRunning: boolean;
	    urlHandler: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DesktopStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], desktop.Settings);
	        this.trayRunning = source["trayRunning"];
	        this.urlHandler = source["urlHandler"];
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
	export class Storage {
	    volumes: library.Volume[];
	    prefixes: epic.PrefixUsage[];
	
	    static createFrom(source: any = {}) {
	        return new Storage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.volumes = this.convertValues(source["volumes"], library.Volume);
	        this.prefixes = this.convertValues(source["prefixes"], epic.PrefixUsage);
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

export namespace sysmon {
	
	export class CPUInfo {
	    model: string;
	    cores: number;
	    threads: number;
	    maxMHz: number;
	    arch: string;
	
	    static createFrom(source: any = {}) {
	        return new CPUInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.cores = source["cores"];
	        this.threads = source["threads"];
	        this.maxMHz = source["maxMHz"];
	        this.arch = source["arch"];
	    }
	}
	export class CPUSample {
	    usage: number;
	    cores: number[];
	    freqMHz: number;
	    tempC?: number;
	    processes: number;
	    threads: number;
	    load: number[];
	    uptime: number;
	
	    static createFrom(source: any = {}) {
	        return new CPUSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.usage = source["usage"];
	        this.cores = source["cores"];
	        this.freqMHz = source["freqMHz"];
	        this.tempC = source["tempC"];
	        this.processes = source["processes"];
	        this.threads = source["threads"];
	        this.load = source["load"];
	        this.uptime = source["uptime"];
	    }
	}
	export class MountInfo {
	    path: string;
	    fs: string;
	    total: number;
	    free: number;
	
	    static createFrom(source: any = {}) {
	        return new MountInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fs = source["fs"];
	        this.total = source["total"];
	        this.free = source["free"];
	    }
	}
	export class DiskInfo {
	    name: string;
	    model: string;
	    size: number;
	    kind: string;
	    mounts: MountInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DiskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.model = source["model"];
	        this.size = source["size"];
	        this.kind = source["kind"];
	        this.mounts = this.convertValues(source["mounts"], MountInfo);
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
	export class DiskSample {
	    name: string;
	    readBps: number;
	    writeBps: number;
	    activePct: number;
	    tempC?: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.readBps = source["readBps"];
	        this.writeBps = source["writeBps"];
	        this.activePct = source["activePct"];
	        this.tempC = source["tempC"];
	    }
	}
	export class GPUInfo {
	    id: string;
	    name: string;
	    driver: string;
	    vramTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new GPUInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.vramTotal = source["vramTotal"];
	    }
	}
	export class GPUSample {
	    id: string;
	    name: string;
	    usage?: number;
	    vramUsed: number;
	    vramTotal: number;
	    tempC?: number;
	    powerW?: number;
	    clockMHz?: number;
	    asleep: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GPUSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.usage = source["usage"];
	        this.vramUsed = source["vramUsed"];
	        this.vramTotal = source["vramTotal"];
	        this.tempC = source["tempC"];
	        this.powerW = source["powerW"];
	        this.clockMHz = source["clockMHz"];
	        this.asleep = source["asleep"];
	    }
	}
	export class HostInfo {
	    hostname: string;
	    os: string;
	    kernel: string;
	
	    static createFrom(source: any = {}) {
	        return new HostInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.os = source["os"];
	        this.kernel = source["kernel"];
	    }
	}
	export class NetInfo {
	    name: string;
	    kind: string;
	    speedMbps: number;
	    addrs: string[];
	
	    static createFrom(source: any = {}) {
	        return new NetInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.speedMbps = source["speedMbps"];
	        this.addrs = source["addrs"];
	    }
	}
	export class MemoryInfo {
	    total: number;
	    swapTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.swapTotal = source["swapTotal"];
	    }
	}
	export class Info {
	    host: HostInfo;
	    cpu: CPUInfo;
	    memory: MemoryInfo;
	    disks: DiskInfo[];
	    nets: NetInfo[];
	    gpus: GPUInfo[];
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = this.convertValues(source["host"], HostInfo);
	        this.cpu = this.convertValues(source["cpu"], CPUInfo);
	        this.memory = this.convertValues(source["memory"], MemoryInfo);
	        this.disks = this.convertValues(source["disks"], DiskInfo);
	        this.nets = this.convertValues(source["nets"], NetInfo);
	        this.gpus = this.convertValues(source["gpus"], GPUInfo);
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
	
	export class MemorySample {
	    total: number;
	    used: number;
	    available: number;
	    free: number;
	    cached: number;
	    buffers: number;
	    swapTotal: number;
	    swapUsed: number;
	
	    static createFrom(source: any = {}) {
	        return new MemorySample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.used = source["used"];
	        this.available = source["available"];
	        this.free = source["free"];
	        this.cached = source["cached"];
	        this.buffers = source["buffers"];
	        this.swapTotal = source["swapTotal"];
	        this.swapUsed = source["swapUsed"];
	    }
	}
	
	
	export class NetSample {
	    name: string;
	    rxBps: number;
	    txBps: number;
	
	    static createFrom(source: any = {}) {
	        return new NetSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.rxBps = source["rxBps"];
	        this.txBps = source["txBps"];
	    }
	}
	export class Sample {
	    time: number;
	    cpu: CPUSample;
	    memory: MemorySample;
	    disks: DiskSample[];
	    nets: NetSample[];
	    gpus: GPUSample[];
	
	    static createFrom(source: any = {}) {
	        return new Sample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.cpu = this.convertValues(source["cpu"], CPUSample);
	        this.memory = this.convertValues(source["memory"], MemorySample);
	        this.disks = this.convertValues(source["disks"], DiskSample);
	        this.nets = this.convertValues(source["nets"], NetSample);
	        this.gpus = this.convertValues(source["gpus"], GPUSample);
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

