export namespace atfile {
	
	export class Candidate {
	    path: string;
	    name: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.dir = source["dir"];
	    }
	}

}

export namespace desktop {
	
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
	export class AgentContextPreview {
	    agent: string;
	    model: string;
	    system: string;
	    charCount: number;
	    skills: string[];
	    persona: Persona;
	    toolCount: number;
	    mcpServers: string[];
	
	    static createFrom(source: any = {}) {
	        return new AgentContextPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agent = source["agent"];
	        this.model = source["model"];
	        this.system = source["system"];
	        this.charCount = source["charCount"];
	        this.skills = source["skills"];
	        this.persona = this.convertValues(source["persona"], Persona);
	        this.toolCount = source["toolCount"];
	        this.mcpServers = source["mcpServers"];
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
	export class AgentDoc {
	    name: string;
	    description: string;
	    model: string;
	    path: string;
	    content: string;
	    builtin: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.model = source["model"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.builtin = source["builtin"];
	        this.error = source["error"];
	    }
	}
	export class AgentForm {
	    name: string;
	    description: string;
	    model: string;
	    fallbackLines: string;
	    tools: string[];
	    skills: string[];
	    mcp: string[];
	    temperature?: number;
	    body: string;
	    hasRouting: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AgentForm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.model = source["model"];
	        this.fallbackLines = source["fallbackLines"];
	        this.tools = source["tools"];
	        this.skills = source["skills"];
	        this.mcp = source["mcp"];
	        this.temperature = source["temperature"];
	        this.body = source["body"];
	        this.hasRouting = source["hasRouting"];
	    }
	}
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
	export class AuditConfigView {
	    backend: string;
	    path: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuditConfigView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.path = source["path"];
	        this.active = source["active"];
	    }
	}
	export class AuditEventView {
	    type: string;
	    sessionId?: string;
	    ts?: string;
	    tool?: string;
	    outcome?: string;
	    error?: string;
	    durationMs?: number;
	    agent?: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEventView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.sessionId = source["sessionId"];
	        this.ts = source["ts"];
	        this.tool = source["tool"];
	        this.outcome = source["outcome"];
	        this.error = source["error"];
	        this.durationMs = source["durationMs"];
	        this.agent = source["agent"];
	        this.model = source["model"];
	    }
	}
	export class ConfigPath {
	    id: string;
	    label: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigPath(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.path = source["path"];
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
	export class EntitlementInfo {
	    plan: string;
	    entitlements: string[];
	    packages: string[];
	    licenseHint: string;
	    isPro: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EntitlementInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plan = source["plan"];
	        this.entitlements = source["entitlements"];
	        this.packages = source["packages"];
	        this.licenseHint = source["licenseHint"];
	        this.isPro = source["isPro"];
	    }
	}
	export class FileResult {
	    path: string;
	    name: string;
	    kind: string;
	    content: string;
	    mimeType?: string;
	    dataB64?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.content = source["content"];
	        this.mimeType = source["mimeType"];
	        this.dataB64 = source["dataB64"];
	    }
	}
	export class HealthCheck {
	    id: string;
	    label: string;
	    status: string;
	    detail: string;
	    fixTab?: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.fixTab = source["fixTab"];
	    }
	}
	export class HealthReport {
	    ok: boolean;
	    checks: HealthCheck[];
	
	    static createFrom(source: any = {}) {
	        return new HealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.checks = this.convertValues(source["checks"], HealthCheck);
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
	export class ImageAttachment {
	    name: string;
	    mimeType: string;
	    dataB64: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageAttachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mimeType = source["mimeType"];
	        this.dataB64 = source["dataB64"];
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
	export class MCPDashboardEntry {
	    name: string;
	    description: string;
	    transport: string;
	    inSession: boolean;
	    reachable: boolean;
	    toolCount: number;
	    tools?: string[];
	    error?: string;
	    configError?: string;
	    checkedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new MCPDashboardEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.transport = source["transport"];
	        this.inSession = source["inSession"];
	        this.reachable = source["reachable"];
	        this.toolCount = source["toolCount"];
	        this.tools = source["tools"];
	        this.error = source["error"];
	        this.configError = source["configError"];
	        this.checkedAt = source["checkedAt"];
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
	export class MCPForm {
	    name: string;
	    description: string;
	    transport: string;
	    command: string[];
	    url: string;
	    toolPrefix: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPForm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.transport = source["transport"];
	        this.command = source["command"];
	        this.url = source["url"];
	        this.toolPrefix = source["toolPrefix"];
	        this.body = source["body"];
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
	export class MCPTestResult {
	    ok: boolean;
	    tools: string[];
	    log: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.tools = source["tools"];
	        this.log = source["log"];
	        this.error = source["error"];
	    }
	}
	export class ToolCall {
	    id: string;
	    name: string;
	    input: string;
	    output?: string;
	    error?: string;
	    duration: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.error = source["error"];
	        this.duration = source["duration"];
	        this.status = source["status"];
	    }
	}
	export class Message {
	    id: string;
	    role: string;
	    content: string;
	    timestamp: number;
	    toolCalls?: ToolCall[];
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.timestamp = source["timestamp"];
	        this.toolCalls = this.convertValues(source["toolCalls"], ToolCall);
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
	export class PackageOffer {
	    name: string;
	    description: string;
	    entitlements: string[];
	    locked: boolean;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackageOffer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.entitlements = source["entitlements"];
	        this.locked = source["locked"];
	        this.installed = source["installed"];
	    }
	}
	
	export class PolicyRuleView {
	    name: string;
	    tool?: string;
	    denyPattern?: string;
	    allowPathPrefix?: string[];
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new PolicyRuleView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tool = source["tool"];
	        this.denyPattern = source["denyPattern"];
	        this.allowPathPrefix = source["allowPathPrefix"];
	        this.message = source["message"];
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
	export class ResumeResult {
	    session: SessionInfo;
	    messages: Message[];
	
	    static createFrom(source: any = {}) {
	        return new ResumeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], SessionInfo);
	        this.messages = this.convertValues(source["messages"], Message);
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
	export class SavedSessionSummary {
	    id: string;
	    label: string;
	    provider: string;
	    model: string;
	    messages: number;
	    preview: string;
	    inputTokens: number;
	    outputTokens: number;
	    savedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new SavedSessionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.messages = source["messages"];
	        this.preview = source["preview"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.savedAt = source["savedAt"];
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
	export class SkillForm {
	    name: string;
	    description: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillForm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.body = source["body"];
	    }
	}
	export class TestResult {
	    ok: boolean;
	    output: string;
	    error?: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.output = source["output"];
	        this.error = source["error"];
	        this.durationMs = source["durationMs"];
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
	export class ToolForm {
	    name: string;
	    description: string;
	    command: string[];
	    timeoutSec: number;
	    parametersJson: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolForm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.command = source["command"];
	        this.timeoutSec = source["timeoutSec"];
	        this.parametersJson = source["parametersJson"];
	        this.body = source["body"];
	    }
	}
	export class UpdateDownloadResult {
	    path: string;
	    filename: string;
	    releaseUrl: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateDownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.filename = source["filename"];
	        this.releaseUrl = source["releaseUrl"];
	        this.notes = source["notes"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    updateAvailable: boolean;
	    pendingInstall: boolean;
	    pendingPath?: string;
	    releaseUrl: string;
	    desktopUrl: string;
	    downloadNotes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.updateAvailable = source["updateAvailable"];
	        this.pendingInstall = source["pendingInstall"];
	        this.pendingPath = source["pendingPath"];
	        this.releaseUrl = source["releaseUrl"];
	        this.desktopUrl = source["desktopUrl"];
	        this.downloadNotes = source["downloadNotes"];
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

