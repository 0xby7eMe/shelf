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

