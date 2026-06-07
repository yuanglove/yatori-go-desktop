export namespace main {
	
	export class AccountListResult {
	    ok: boolean;
	    data: service.AccountVO[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], service.AccountVO);
	        this.error = source["error"];
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
	export class BoolResult {
	    ok: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BoolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	    }
	}
	export class ConfigResult {
	    ok: boolean;
	    data: service.AppConfig;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], service.AppConfig);
	        this.error = source["error"];
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
	export class DashboardResult {
	    ok: boolean;
	    data: service.Dashboard;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DashboardResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], service.Dashboard);
	        this.error = source["error"];
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
	export class PlatformListResult {
	    ok: boolean;
	    data: service.PlatformInfo[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PlatformListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], service.PlatformInfo);
	        this.error = source["error"];
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
	export class StringListResult {
	    ok: boolean;
	    data: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new StringListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = source["data"];
	        this.error = source["error"];
	    }
	}
	export class StringResult {
	    ok: boolean;
	    data: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new StringResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = source["data"];
	        this.error = source["error"];
	    }
	}
	export class TaskStatusListResult {
	    ok: boolean;
	    data: service.TaskStatus[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskStatusListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], service.TaskStatus);
	        this.error = source["error"];
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
	export class UpdateInfo {
	    hasUpdate: boolean;
	    latestVersion: string;
	    currentVersion: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.latestVersion = source["latestVersion"];
	        this.currentVersion = source["currentVersion"];
	        this.url = source["url"];
	    }
	}
	export class UpdateResult {
	    ok: boolean;
	    data: UpdateInfo;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], UpdateInfo);
	        this.error = source["error"];
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

export namespace service {
	
	export class CourseSettings {
	    name: string;
	    includeExams?: string[];
	    excludeExams?: string[];
	
	    static createFrom(source: any = {}) {
	        return new CourseSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.includeExams = source["includeExams"];
	        this.excludeExams = source["excludeExams"];
	    }
	}
	export class CoursesCustom {
	    studyTime?: string;
	    cxNode?: number;
	    cxChapterTestSw?: number;
	    cxWorkSw?: number;
	    cxExamSw?: number;
	    shuffleSw: number;
	    videoModel: number;
	    autoExam: number;
	    examAutoSubmit: number;
	    submitThresholdPercent?: number;
	    excludeCourses: string[];
	    includeCourses: string[];
	    coursesSettings?: CourseSettings[];
	
	    static createFrom(source: any = {}) {
	        return new CoursesCustom(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.studyTime = source["studyTime"];
	        this.cxNode = source["cxNode"];
	        this.cxChapterTestSw = source["cxChapterTestSw"];
	        this.cxWorkSw = source["cxWorkSw"];
	        this.cxExamSw = source["cxExamSw"];
	        this.shuffleSw = source["shuffleSw"];
	        this.videoModel = source["videoModel"];
	        this.autoExam = source["autoExam"];
	        this.examAutoSubmit = source["examAutoSubmit"];
	        this.submitThresholdPercent = source["submitThresholdPercent"];
	        this.excludeCourses = source["excludeCourses"];
	        this.includeCourses = source["includeCourses"];
	        this.coursesSettings = this.convertValues(source["coursesSettings"], CourseSettings);
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
	export class AccountReq {
	    uid: string;
	    accountType: string;
	    url: string;
	    remarkName: string;
	    account: string;
	    password: string;
	    isProxy: number;
	    informEmails: string[];
	    coursesCustom: CoursesCustom;
	
	    static createFrom(source: any = {}) {
	        return new AccountReq(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.accountType = source["accountType"];
	        this.url = source["url"];
	        this.remarkName = source["remarkName"];
	        this.account = source["account"];
	        this.password = source["password"];
	        this.isProxy = source["isProxy"];
	        this.informEmails = source["informEmails"];
	        this.coursesCustom = this.convertValues(source["coursesCustom"], CoursesCustom);
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
	export class AccountVO {
	    uid: string;
	    accountType: string;
	    url: string;
	    remarkName: string;
	    account: string;
	    isProxy: number;
	    isRunning: boolean;
	    guiSupport: string;
	    coursesCustom: CoursesCustom;
	    informEmails: string[];
	
	    static createFrom(source: any = {}) {
	        return new AccountVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.accountType = source["accountType"];
	        this.url = source["url"];
	        this.remarkName = source["remarkName"];
	        this.account = source["account"];
	        this.isProxy = source["isProxy"];
	        this.isRunning = source["isRunning"];
	        this.guiSupport = source["guiSupport"];
	        this.coursesCustom = this.convertValues(source["coursesCustom"], CoursesCustom);
	        this.informEmails = source["informEmails"];
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
	export class AiSetting {
	    aiType: string;
	    aiUrl: string;
	    model: string;
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new AiSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.aiType = source["aiType"];
	        this.aiUrl = source["aiUrl"];
	        this.model = source["model"];
	        this.apiKey = source["apiKey"];
	    }
	}
	export class ApiQueSetting {
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new ApiQueSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	    }
	}
	export class User {
	    accountType: string;
	    url: string;
	    remarkName?: string;
	    account: string;
	    password: string;
	    isProxy: number;
	    informEmails: string[];
	    coursesCustom: CoursesCustom;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountType = source["accountType"];
	        this.url = source["url"];
	        this.remarkName = source["remarkName"];
	        this.account = source["account"];
	        this.password = source["password"];
	        this.isProxy = source["isProxy"];
	        this.informEmails = source["informEmails"];
	        this.coursesCustom = this.convertValues(source["coursesCustom"], CoursesCustom);
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
	export class EmailInform {
	    sw: number;
	    smtpHost: string;
	    smtpPort: number;
	    userName: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new EmailInform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sw = source["sw"];
	        this.smtpHost = source["smtpHost"];
	        this.smtpPort = source["smtpPort"];
	        this.userName = source["userName"];
	        this.password = source["password"];
	    }
	}
	export class BasicSetting {
	    completionTone: number;
	    colorLog: number;
	    logOutFileSw: number;
	    logLevel: string;
	    logModel: number;
	    webModel: number;
	    theme?: string;
	    maxWorkers?: number;
	
	    static createFrom(source: any = {}) {
	        return new BasicSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.completionTone = source["completionTone"];
	        this.colorLog = source["colorLog"];
	        this.logOutFileSw = source["logOutFileSw"];
	        this.logLevel = source["logLevel"];
	        this.logModel = source["logModel"];
	        this.webModel = source["webModel"];
	        this.theme = source["theme"];
	        this.maxWorkers = source["maxWorkers"];
	    }
	}
	export class Setting {
	    basicSetting: BasicSetting;
	    emailInform: EmailInform;
	    aiSetting: AiSetting;
	    apiQueSetting: ApiQueSetting;
	
	    static createFrom(source: any = {}) {
	        return new Setting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.basicSetting = this.convertValues(source["basicSetting"], BasicSetting);
	        this.emailInform = this.convertValues(source["emailInform"], EmailInform);
	        this.aiSetting = this.convertValues(source["aiSetting"], AiSetting);
	        this.apiQueSetting = this.convertValues(source["apiQueSetting"], ApiQueSetting);
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
	export class AppConfig {
	    setting: Setting;
	    users: User[];
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setting = this.convertValues(source["setting"], Setting);
	        this.users = this.convertValues(source["users"], User);
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
	
	
	
	export class Dashboard {
	    totalAccounts: number;
	    runningTasks: number;
	    configPath: string;
	    configOK: boolean;
	    recentLogs: string[];
	
	    static createFrom(source: any = {}) {
	        return new Dashboard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalAccounts = source["totalAccounts"];
	        this.runningTasks = source["runningTasks"];
	        this.configPath = source["configPath"];
	        this.configOK = source["configOK"];
	        this.recentLogs = source["recentLogs"];
	    }
	}
	
	export class PlatformInfo {
	    code: string;
	    name: string;
	    guiSupport: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new PlatformInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.guiSupport = source["guiSupport"];
	        this.note = source["note"];
	    }
	}
	
	export class TaskStatus {
	    uid: string;
	    account: string;
	    platform: string;
	    state: string;
	    startTime?: string;
	    lastLog?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.account = source["account"];
	        this.platform = source["platform"];
	        this.state = source["state"];
	        this.startTime = source["startTime"];
	        this.lastLog = source["lastLog"];
	        this.error = source["error"];
	    }
	}

}

