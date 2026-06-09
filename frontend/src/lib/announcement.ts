// 远程公告拉取与已读管理

export const ANNOUNCEMENT_URL = 'https://raw.githubusercontent.com/yuanglove/yatori-go-desktop/main/announcement.json'

const READ_KEY = 'yatori-read-announcement-id'

export interface AnnouncementData {
  id: string
  title: string
  content: string
  level: 'info' | 'warn' | 'error'
  enabled: boolean
  updatedAt: string
  url?: string
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
  if (!isValidHttpUrl(ANNOUNCEMENT_URL)) return null
  try {
    const res = await fetch(ANNOUNCEMENT_URL, { signal: AbortSignal.timeout(6000) })
    if (!res.ok) return null
    const data: AnnouncementData = await res.json()
    if (!data.id || !data.title || !data.content) return null
    if (!data.enabled) return null
    return data
  } catch {
    // Network failures are ignored so startup never shows a blank page.
    return null
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
