import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export interface AppConfig {
  setting: {
    basicSetting:  { completionTone:number; colorLog:number; logOutFileSw:number; logLevel:string; logModel:number; webModel:number; theme?:string; maxWorkers?:number }
    emailInform:   { sw:number; smtpHost:string; smtpPort:number; userName:string; password:string }
    aiSetting:     { aiType:string; aiUrl:string; model:string; apiKey:string }
    apiQueSetting: {
      url:string; protocol?:string; token?:string; tokenParam?:string; authType?:string
      method?:string; contentType?:string; questionField?:string; typeField?:string
      optionsField?:string; courseNameField?:string; optionsFormat?:string; typeMap?:string
      answerPath?:string; answerSplit?:string
    }
  }
  users: User[]
}
export interface User {
  accountType:string; url:string; remarkName?:string; account:string; password:string
  isProxy:number; informEmails:string[]; coursesCustom:CoursesCustom
}
export interface CoursesCustom {
  studyTime?:string; cxNode?:number; cxChapterTestSw?:number; cxWorkSw?:number; cxExamSw?:number
  shuffleSw:number; videoModel:number; autoExam:number; examAutoSubmit:number
  submitThresholdPercent?:number; randomAnswerOnFail?:number
  xxtAutoExamCode?:number
  xxtExamCodes?:Record<string,string>
  excludeCourses:string[]; includeCourses:string[]
}
export interface AccountVO {
  uid:string; accountType:string; url:string; remarkName?:string; account:string
  isProxy:number; isRunning:boolean; guiSupport:'full'|'config-only'|'none'
  coursesCustom:CoursesCustom; informEmails:string[]
}
export type AccountReq = Omit<AccountVO, 'isRunning'|'guiSupport'> & { password:string }
export interface TaskStatus {
  uid:string; account:string; platform:string; state:'running'|'stopped'|'failed'
  startTime?:string; lastLog?:string; error?:string
}
export interface Dashboard { totalAccounts:number; runningTasks:number; configPath:string; configOK:boolean; recentLogs:string[] }
export interface PlatformInfo { code:string; name:string; guiSupport:string; note:string }
export interface UpdateInfo { hasUpdate:boolean; latestVersion:string; currentVersion:string; url:string }
export interface CourseVO { platform:string; key:string; courseId:string; courseName:string; courseTeacher:string; jobFinishCount:number; jobCount:number; jobRate:number; hasProgress:boolean; state:number; isStart:boolean; rawStatusText:string }
export interface XXTExamCodeRequest { id:string; uid:string; account:string; examName:string; taskRefId:string; createdAt:number; expiresAt:number; status:string; code?:string }

export interface BoolResult           { ok:boolean; error?:string }
export interface StringResult         { ok:boolean; data:string;         error?:string }
export interface ConfigResult         { ok:boolean; data:AppConfig;      error?:string }
export interface AccountListResult    { ok:boolean; data:AccountVO[];    error?:string }
export interface TaskStatusListResult { ok:boolean; data:TaskStatus[];   error?:string }
export interface DashboardResult      { ok:boolean; data:Dashboard;      error?:string }
export interface StringListResult     { ok:boolean; data:string[];       error?:string }
export interface PlatformListResult   { ok:boolean; data:PlatformInfo[]; error?:string }
export interface UpdateResult         { ok:boolean; data:UpdateInfo;     error?:string }
export interface CourseListResult     { ok:boolean; data:CourseVO[];     error?:string }
export interface XXTExamCodeRequestListResult { ok:boolean; data:XXTExamCodeRequest[]; error?:string }

const c = <T>(p: Promise<unknown>): Promise<T> => p as Promise<T>

export const api = {
  getConfig:          ():                Promise<ConfigResult>          => c(App.GetConfig()),
  saveConfig:         (v: AppConfig):    Promise<BoolResult>            => c(App.SaveConfig(v as never)),
  getDataDir:         ():                Promise<StringResult>          => c(App.GetDataDir()),
  openDataDir:        ():                Promise<BoolResult>            => c(App.OpenDataDir()),
  importConfig:       ():                Promise<ConfigResult>          => c(App.ImportConfig()),
  exportConfig:       (v: AppConfig):    Promise<BoolResult>            => c(App.ExportConfig(v as never)),
  listAccounts:       ():                Promise<AccountListResult>     => c(App.ListAccounts()),
  addAccount:         (v: AccountReq):   Promise<BoolResult>            => c(App.AddAccount(v as never)),
  updateAccount:      (v: AccountReq):   Promise<BoolResult>            => c(App.UpdateAccount(v as never)),
  deleteAccount:      (uid: string):     Promise<BoolResult>            => c(App.DeleteAccount(uid)),
  startTask:          (uid: string):     Promise<BoolResult>            => c(App.StartTask(uid)),
  stopTask:           (uid: string):     Promise<BoolResult>            => c(App.StopTask(uid)),
  getTaskStatuses:    ():                Promise<TaskStatusListResult>  => c(App.GetTaskStatuses()),
  getDashboard:       ():                Promise<DashboardResult>       => c(App.GetDashboard()),
  tailLog:            (n: number):       Promise<StringListResult>      => c(App.TailLogFile(n)),
  getRecentLogs:      (n: number):       Promise<StringListResult>      => c(App.GetRecentLogs(n)),
  getPlatformSupport: ():                Promise<PlatformListResult>    => c(App.GetPlatformSupport()),
  testAIConfig:       ():                Promise<StringResult>          => c(App.TestAIConfig()),
  testQuestionBankConfig: ():            Promise<StringResult>          => c(App.TestQuestionBankConfig()),
  checkForUpdates:    (v: string):       Promise<UpdateResult>          => c(App.CheckForUpdates(v)),
  openURL:            (url: string):     Promise<BoolResult>            => c(App.OpenURL(url)),
  startICVECookieCapture: (url: string): Promise<StringResult>          => c(App.StartICVECookieCapture(url)),
  readICVECookie:     ():                Promise<StringResult>          => c(App.ReadICVECookie()),
  getCourses:         (uid: string):     Promise<CourseListResult>      => c(App.GetCourses(uid)),
  listXXTExamCodeRequests: ():           Promise<XXTExamCodeRequestListResult> => c(App.ListXXTExamCodeRequests()),
  answerXXTExamCodeRequest: (id: string, code: string): Promise<BoolResult> => c(App.AnswerXXTExamCodeRequest(id, code)),
  cancelXXTExamCodeRequest: (id: string): Promise<BoolResult> => c(App.CancelXXTExamCodeRequest(id)),
}

export function onTaskLog(uid: string, cb: (msg: string) => void): () => void {
  return EventsOn('log:' + uid, cb)
}
export function onAnyLog(cb: (item: { uid: string; msg: string }) => void): () => void {
  return EventsOn('log:all', cb as (...args: unknown[]) => void)
}
