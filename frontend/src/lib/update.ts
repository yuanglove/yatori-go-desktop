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
    return info.hasUpdate ? info : undefined
  } catch {
    // 自动检查失败时保持静默，避免影响启动。
  }
}
