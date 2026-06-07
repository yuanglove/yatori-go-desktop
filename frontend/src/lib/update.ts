import { APP_VERSION, PROJECT_RELEASES_URL, PROJECT_REPO_API_URL } from './version'

export interface UpdateInfo {
  hasUpdate: boolean
  latestVersion: string
  currentVersion: string
  url: string
}

function normalizeVersion(v: string): string {
  return v.trim().replace(/^v/i, '')
}

function compareVersions(a: string, b: string): number {
  const pa = normalizeVersion(a).split('.').map(n => Number.parseInt(n, 10) || 0)
  const pb = normalizeVersion(b).split('.').map(n => Number.parseInt(n, 10) || 0)
  const len = Math.max(pa.length, pb.length)
  for (let i = 0; i < len; i++) {
    const diff = (pa[i] || 0) - (pb[i] || 0)
    if (diff !== 0) return diff > 0 ? 1 : -1
  }
  return 0
}

async function fetchLatestVersion(): Promise<{ version: string; url: string }> {
  const releaseResp = await fetch(`${PROJECT_REPO_API_URL}/releases/latest`, { cache: 'no-store' })
  if (releaseResp.ok) {
    const release = await releaseResp.json()
    const tag = String(release.tag_name || '')
    if (tag) return { version: tag, url: String(release.html_url || PROJECT_RELEASES_URL) }
  }

  const tagsResp = await fetch(`${PROJECT_REPO_API_URL}/tags`, { cache: 'no-store' })
  if (!tagsResp.ok) throw new Error('无法获取 GitHub 版本信息')
  const tags = await tagsResp.json()
  const first = Array.isArray(tags) ? tags[0] : null
  const tag = first?.name ? String(first.name) : ''
  if (!tag) throw new Error('GitHub 暂无可用版本标签')
  return { version: tag, url: PROJECT_RELEASES_URL }
}

export async function checkForUpdates(): Promise<UpdateInfo> {
  const latest = await fetchLatestVersion()
  return {
    hasUpdate: compareVersions(latest.version, APP_VERSION) > 0,
    latestVersion: normalizeVersion(latest.version),
    currentVersion: normalizeVersion(APP_VERSION),
    url: latest.url,
  }
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
