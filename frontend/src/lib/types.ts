export interface Result<T> { ok: boolean; data: T; error?: string }

export interface AppConfig { setting: Setting; users: User[] }
export interface Setting {
  basicSetting: BasicSetting
  emailInform: EmailInform
  aiSetting: AiSetting
  apiQueSetting: { url: string }
}
export interface BasicSetting {
  completionTone: number; colorLog: number; logOutFileSw: number
  logLevel: string; logModel: number; webModel: number
}
export interface EmailInform {
  sw: number; smtpHost: string; smtpPort: number; userName: string; password: string
}
export interface AiSetting { aiType: string; aiUrl: string; model: string; apiKey: string }
export interface User {
  accountType: string; url: string; remarkName?: string
  account: string; password: string; isProxy: number
  informEmails: string[]; coursesCustom: CoursesCustom
}
export interface CoursesCustom {
  studyTime?: string; cxNode?: number; cxChapterTestSw?: number
  cxWorkSw?: number; cxExamSw?: number; shuffleSw: number
  videoModel: number; autoExam: number; examAutoSubmit: number
  excludeCourses: string[]; includeCourses: string[]
}
export interface AccountVO {
  uid: string; accountType: string; url: string; remarkName?: string
  account: string; isProxy: number; isRunning: boolean
  guiSupport: 'full' | 'config-only' | 'none'
  coursesCustom: CoursesCustom; informEmails: string[]
}
export type AccountReq = Omit<AccountVO, 'isRunning' | 'guiSupport'> & { password: string }
export interface TaskStatus {
  uid: string; account: string; platform: string
  state: 'running' | 'stopped' | 'failed'
  startTime?: string; lastLog?: string; error?: string
}
export interface Dashboard {
  totalAccounts: number; runningTasks: number
  configPath: string; configOK: boolean; recentLogs: string[]
}
export interface PlatformInfo { code: string; name: string; guiSupport: string; note: string }
