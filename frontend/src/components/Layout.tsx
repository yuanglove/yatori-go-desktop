import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { api, type UpdateInfo } from '../lib/api'
import { autoCheckForUpdates } from '../lib/update'
import { applyTheme } from '../lib/theme'
import { PROJECT_RELEASES_URL } from '../lib/version'

const links = [
  { to: '/',         label: '仪表盘',   icon: '⬡' },
  { to: '/accounts', label: '账号管理', icon: '◈' },
  { to: '/tasks',    label: '任务控制', icon: '▷' },
  { to: '/courses',  label: '课程进度', icon: '◎' },
  { to: '/logs',     label: '日志中心', icon: '≡' },
  { to: '/settings', label: '全局设置', icon: '⚙' },
  { to: '/about',    label: '关于',     icon: 'ⓘ' },
]

export default function Layout() {
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)

  useEffect(() => {
    applyTheme(localStorage.getItem('yatori-theme') || 'dark')
    api.getConfig().then(r => {
      if (r.ok) applyTheme(r.data.setting.basicSetting.theme || 'dark')
    }).catch(() => applyTheme(localStorage.getItem('yatori-theme') || 'dark'))
    autoCheckForUpdates().then(info => {
      if (info) setUpdateInfo(info)
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
          <div className="nav-logo-title">Yatori</div>
          <small>学习管理工具</small>
        </div>
        <div className="nav-links">
          {links.map(l => (
            <NavLink
              key={l.to}
              to={l.to}
              end={l.to === '/'}
              className={({ isActive }) => 'nav-link' + (isActive ? ' active' : '')}
            >
              <span className="nav-icon">{l.icon}</span>
              {l.label}
            </NavLink>
          ))}
        </div>
        <div className="nav-footer">
          仅用于本人授权账号<br />合规使用
        </div>
      </nav>

      <main className="main">
        <Outlet />
      </main>

      {updateInfo && (
        <div className="update-toast" role="dialog" aria-label="发现新版本">
          <div className="update-toast-title">🔔 发现新版本</div>
          <div className="update-toast-body">
            当前 v{updateInfo.currentVersion}，最新 v{updateInfo.latestVersion}
          </div>
          <div className="update-toast-actions">
            <button className="btn btn-ghost btn-sm" onClick={() => setUpdateInfo(null)}>稍后</button>
            <button className="btn btn-primary btn-sm" onClick={openUpdate}>去更新</button>
          </div>
        </div>
      )}
    </div>
  )
}
