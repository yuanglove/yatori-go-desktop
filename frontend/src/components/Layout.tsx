import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import {
  LayoutDashboard, Users, Play, BookOpen, ScrollText, Settings, Info, Bell
} from 'lucide-react'
import { api, type UpdateInfo } from '../lib/api'
import { autoCheckForUpdates } from '../lib/update'
import { applyTheme } from '../lib/theme'
import { PROJECT_RELEASES_URL } from '../lib/version'
import { fetchAnnouncement, isAnnouncementRead, type AnnouncementData } from '../lib/announcement'
import AnnouncementModal from './AnnouncementModal'

const links = [
  { to: '/',         label: '仪表盘',   Icon: LayoutDashboard },
  { to: '/accounts', label: '账号管理', Icon: Users },
  { to: '/tasks',    label: '任务控制', Icon: Play },
  { to: '/courses',  label: '课程进度', Icon: BookOpen },
  { to: '/logs',     label: '日志中心', Icon: ScrollText },
  { to: '/settings', label: '全局设置', Icon: Settings },
  { to: '/about',    label: '关于',           Icon: Info },
]

export default function Layout() {
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [announcement, setAnnouncement] = useState<AnnouncementData | null>(null)

  useEffect(() => {
    applyTheme(localStorage.getItem('yatori-theme') || 'dark')
    api.getConfig().then(r => {
      if (r.ok) applyTheme(r.data.setting.basicSetting.theme || 'dark')
    }).catch(() => applyTheme(localStorage.getItem('yatori-theme') || 'dark'))
    autoCheckForUpdates().then(info => {
      if (info) setUpdateInfo(info)
    })
    fetchAnnouncement().then(data => {
      if (data && !isAnnouncementRead(data.id)) setAnnouncement(data)
    })
  }, [])

  const openUpdate = () => {
    api.openURL(updateInfo?.url || PROJECT_RELEASES_URL)
    setUpdateInfo(null)
  }

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-logo">
          <div className="nav-logo-title">{'Yatori'}</div>
          <small>{'学习管理工具'}</small>
        </div>
        <div className="nav-links">
          {links.map(({ to, label, Icon }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) => 'nav-link' + (isActive ? ' active' : '')}
            >
              <span className="nav-icon">
                <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
              </span>
              {label}
            </NavLink>
          ))}
        </div>
        <div className="nav-footer">
          {'仅用于本人授权账号'}<br />{'合规使用'}
        </div>
      </nav>

      <main className="main">
        <Outlet />
      </main>

      {updateInfo && (
        <div className="update-toast" role="dialog" aria-label={'发现新版本'}>
          <div className="update-toast-title">
            <Bell size={13} strokeWidth={2} style={{ verticalAlign: 'middle', marginRight: 5 }} />
            {'发现新版本'}
          </div>
          <div className="update-toast-body">
            {'当前 v'}{updateInfo.currentVersion}{'，最新 v'}{updateInfo.latestVersion}
          </div>
          <div className="update-toast-actions">
            <button className="btn btn-ghost btn-sm" onClick={() => setUpdateInfo(null)}>{'稍后'}</button>
            <button className="btn btn-primary btn-sm" onClick={openUpdate}>{'去更新'}</button>
          </div>
        </div>
      )}

      {announcement && (
        <AnnouncementModal
          data={announcement}
          onClose={() => setAnnouncement(null)}
          openURL={url => api.openURL(url)}
        />
      )}
    </div>
  )
}
