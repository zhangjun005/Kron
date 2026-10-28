export namespace main {

	export class GUIProjectSettings {
	    displayName: string;
	    coverImage: string;

	    static createFrom(source: any = {}) {
	        return new GUIProjectSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayName = source["displayName"];
	        this.coverImage = source["coverImage"];
	    }
	}

	export class GUISettings {
	    schemaVersion: number;
	    theme: string;
	    projects: {[key: string]: GUIProjectSettings};

	    static createFrom(source: any = {}) {
	        return new GUISettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.theme = source["theme"];
	        this.projects = this.convertValues(source["projects"], GUIProjectSettings, true);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (asMap && "object" === typeof a) {
		        for (const key of Object.keys(a)) {
		            a[key] = new classs(a[key]);
		        }
		        return a;
		    }
		    return new classs(a);
		}
	}

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
	export class ProjectEntry {
	    path: string;
	    addedAt?: string;
	    lastOpened?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.addedAt = source["addedAt"];
	        this.lastOpened = source["lastOpened"];
	    }
	}
	export class ListProjectsLiveResult {
	    projects: ProjectEntry[];
	    current: string;
	    projectsPath: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new ListProjectsLiveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = this.convertValues(source["projects"], ProjectEntry);
	        this.current = source["current"];
	        this.projectsPath = source["projectsPath"];
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
	
	
	export class OpenIDEResult {
	    ok: boolean;
	    ide: string;
	    pid?: number;
	    err?: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenIDEResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.ide = source["ide"];
	        this.pid = source["pid"];
	        this.err = source["err"];
	        this.path = source["path"];
	    }
	}

}

