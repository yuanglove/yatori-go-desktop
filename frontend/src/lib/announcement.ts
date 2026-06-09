// 远程公告拉取与已读管理

export const ANNOUNCEMENT_URL = 'https://raw.githubusercontent.com/yuanglove/yatori-go-desktop/main/announcement.json'

const READ_KEY = 'yatori-read-announcement-id'
const FETCH_TIMEOUT_MS = 6000

export interface AnnouncementData {
  id: string
  title: string
  content: string
  level: 'info' | 'warn' | 'error'
  enabled: boolean
  updatedAt: string
  url?: string
}

export const DEFAULT_ANNOUNCEMENT: AnnouncementData = {
  id: '2026-06-09-disclaimer-v2',
  title: '免责声明',
  content: '本项目代码已开源，仅供学习与技术交流使用，严禁任何形式的倒卖、贩卖或商业牟利。\n\n请在遵守法律法规、平台规则和账号授权范围的前提下使用本软件。任何个人或组织使用本项目代码或软件进行的违法违规行为，均与项目作者无关，相关责任由使用者自行承担。\n\n如本项目内容对相关公司或平台造成影响，请通过 GitHub 仓库联系，我会及时处理。',
  level: 'warn',
  enabled: true,
  updatedAt: '2026-06-09 18:00:00',
  url: 'https://github.com/yuanglove/yatori-go-desktop',
}

function isValidHttpUrl(s: string): boolean {
  try {
    const u = new URL(s)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

export async function fetchAnnouncement(): Promise<AnnouncementData | null> {
  if (!isValidHttpUrl(ANNOUNCEMENT_URL)) return DEFAULT_ANNOUNCEMENT
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS)
  try {
    const res = await fetch(ANNOUNCEMENT_URL, {
      signal: controller.signal,
      cache: 'no-store',
    })
    if (!res.ok) return DEFAULT_ANNOUNCEMENT
    const data: AnnouncementData = await res.json()
    if (!data.id || !data.title || !data.content) return DEFAULT_ANNOUNCEMENT
    if (!data.enabled) return null
    return data
  } catch {
    return DEFAULT_ANNOUNCEMENT
  } finally {
    window.clearTimeout(timer)
  }
}

export function markAnnouncementRead(id: string): void {
  try {
    localStorage.setItem(READ_KEY, id)
  } catch {
    // ignore
  }
}

export function isAnnouncementRead(id: string): boolean {
  try {
    return localStorage.getItem(READ_KEY) === id
  } catch {
    return false
  }
}
