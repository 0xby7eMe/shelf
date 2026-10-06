export namespace library {
	
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
	    }
	}

}

