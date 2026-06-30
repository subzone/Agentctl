export namespace desktop {
	
	export class AgentInfo {
	    name: string;
	    description: string;
	    model: string;
	    path: string;
	    builtin: boolean;
	    category: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.model = source["model"];
	        this.path = source["path"];
	        this.builtin = source["builtin"];
	        this.category = source["category"];
	    }
	}
	export class CostInfo {
	    inputTokens: number;
	    outputTokens: number;
	    cost: number;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new CostInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cost = source["cost"];
	        this.model = source["model"];
	    }
	}
	export class FileResult {
	    path: string;
	    name: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new FileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class KMLink {
	    source: string;
	    target: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new KMLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.type = source["type"];
	    }
	}
	export class KMNode {
	    id: string;
	    type: string;
	    label: string;
	    category?: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new KMNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.label = source["label"];
	        this.category = source["category"];
	        this.source = source["source"];
	    }
	}
	export class KMGraphData {
	    nodes: KMNode[];
	    links: KMLink[];
	
	    static createFrom(source: any = {}) {
	        return new KMGraphData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], KMNode);
	        this.links = this.convertValues(source["links"], KMLink);
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
	export class KMHealth {
	    available: boolean;
	    baseUrl: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new KMHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.baseUrl = source["baseUrl"];
	        this.error = source["error"];
	    }
	}
	
	
	export class KMStats {
	    chunks: number;
	    documents: number;
	    repos: number;
	
	    static createFrom(source: any = {}) {
	        return new KMStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunks = source["chunks"];
	        this.documents = source["documents"];
	        this.repos = source["repos"];
	    }
	}
	export class MCPDoc {
	    name: string;
	    description: string;
	    transport: string;
	    path: string;
	    content: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.transport = source["transport"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.error = source["error"];
	    }
	}
	export class MCPStatus {
	    name: string;
	    transport: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.transport = source["transport"];
	        this.installed = source["installed"];
	    }
	}
	export class Persona {
	    instructions: string;
	    tone: string;
	    verbosity: string;
	    temperature?: number;
	
	    static createFrom(source: any = {}) {
	        return new Persona(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instructions = source["instructions"];
	        this.tone = source["tone"];
	        this.verbosity = source["verbosity"];
	        this.temperature = source["temperature"];
	    }
	}
	export class SessionInfo {
	    id: string;
	    agent: string;
	    model: string;
	    messages: number;
	    createdAt: number;
	    preview: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.agent = source["agent"];
	        this.model = source["model"];
	        this.messages = source["messages"];
	        this.createdAt = source["createdAt"];
	        this.preview = source["preview"];
	    }
	}
	export class SkillDoc {
	    name: string;
	    description: string;
	    tools: string[];
	    path: string;
	    content: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.tools = source["tools"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.error = source["error"];
	    }
	}
	export class ThemeInfo {
	    name: string;
	    bg: string;
	    bgPanel: string;
	    bgInput: string;
	    border: string;
	    user: string;
	    assistant: string;
	    tool: string;
	    error: string;
	    accent: string;
	    text: string;
	    muted: string;
	
	    static createFrom(source: any = {}) {
	        return new ThemeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.bg = source["bg"];
	        this.bgPanel = source["bgPanel"];
	        this.bgInput = source["bgInput"];
	        this.border = source["border"];
	        this.user = source["user"];
	        this.assistant = source["assistant"];
	        this.tool = source["tool"];
	        this.error = source["error"];
	        this.accent = source["accent"];
	        this.text = source["text"];
	        this.muted = source["muted"];
	    }
	}
	export class ToolDoc {
	    name: string;
	    description: string;
	    runtime: string;
	    path: string;
	    content: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.runtime = source["runtime"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.error = source["error"];
	    }
	}

}

export namespace userconfig {
	
	export class Config {
	    Provider: string;
	    Model: string;
	    BaseURL: string;
	    DefaultAgent: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Provider = source["Provider"];
	        this.Model = source["Model"];
	        this.BaseURL = source["BaseURL"];
	        this.DefaultAgent = source["DefaultAgent"];
	    }
	}

}

