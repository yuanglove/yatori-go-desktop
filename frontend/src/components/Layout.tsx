import { useEffect } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { api } from '../lib/api'
import { autoCheckForUpdates } from '../lib/update'
import { applyTheme } from '../lib/theme'

const links = [
  { to: '/', label: '仪表盘', icon: '▦' },
  { to: '/accounts', label: '账号管理', icon: '👤' },
  { to: '/tasks', label: '任务控制', icon: '▶' },
  { to: '/logs', label: '日志中心', icon: '≡' },
  { to: '/settings', label: '全局设置', icon: '⚙' },
  { to: '/about', label: '关于本项目', icon: 'ⓘ' },
]

export default function Layout() {
  useEffect(() => {
    applyTheme(localStorage.getItem('yatori-theme') || 'dark')
    api.getConfig().then(r => {
      if (r.ok) applyTheme(r.data.setting.basicSetting.theme || 'dark')
    }).catch(() => applyTheme(localStorage.getItem('yatori-theme') || 'dark'))
    autoCheckForUpdates()
  }, [])

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-logo">
          Yatori
          <small>学习管理工具</small>
        </div>
        <div className="nav-links">
          {links.map(l => (
            <NavLink key={l.to} to={l.to} end={l.to === '/'} className={({ isActive }) => 'nav-link' + (isActive ? ' active' : '')}>
              <span className="nav-icon">{l.icon}</span>
              {l.label}
            </NavLink>
          ))}
        </div>
        <div style={{ padding: '12px 14px', fontSize: 11, color: 'var(--text2)', borderTop: '1px solid var(--border)' }}>
          仅用于本人授权账号<br />合规学习管理
        </div>
      </nav>
      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
