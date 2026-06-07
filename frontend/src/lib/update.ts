import { api, type UpdateInfo } from './api'
import { APP_VERSION } from './version'

export type { UpdateInfo }

export async function checkForUpdates(): Promise<UpdateInfo> {
  const result = await api.checkForUpdates(APP_VERSION)
  if (!result.ok) throw new Error(result.error || '无法获取 GitHub 版本信息')
  return result.data
}

export async function autoCheckForUpdates() {
  if (sessionStorage.getItem('yatori-update-checked') === '1') return
  sessionStorage.setItem('yatori-update-checked', '1')
  try {
    const info = await checkForUpdates()
    if (info.hasUpdate) {
      window.alert(`发现新版本 v${info.latestVersion}\n当前版本 v${info.currentVersion}\n请在“关于本项目”页面打开更新日志下载新版。`)
    }
  } catch {
    // 自动检查失败时保持静默，避免影响启动。
  }
}
