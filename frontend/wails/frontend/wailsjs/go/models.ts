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
	export class LiveServerInfo {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new LiveServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class LiveMCPTool {
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new LiveMCPTool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	    }
	}
	export class ListMCPToolsLiveResult {
	    tools: LiveMCPTool[];
	    serverInfo: LiveServerInfo;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new ListMCPToolsLiveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tools = this.convertValues(source["tools"], LiveMCPTool);
	        this.serverInfo = this.convertValues(source["serverInfo"], LiveServerInfo);
	        this.source = source["source"];
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

