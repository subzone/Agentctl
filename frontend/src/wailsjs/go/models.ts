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

