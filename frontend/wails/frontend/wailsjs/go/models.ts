export namespace main {
	
	export class KronInfo {
	    name: string;
	    version: string;
	    buildTime: string;
	    goVersion: string;
	    os: string;
	    arch: string;
	    description: string;
	    rfc: string;
	
	    static createFrom(source: any = {}) {
	        return new KronInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.buildTime = source["buildTime"];
	        this.goVersion = source["goVersion"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.description = source["description"];
	        this.rfc = source["rfc"];
	    }
	}

}

