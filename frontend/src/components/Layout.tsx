import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import {
  LayoutDashboard, Users, Play, BookOpen, ScrollText, Settings, Info, Bell
} from 'lucide-react'
import { api, type UpdateInfo, type XXTExamCodeRequest } from '../lib/api'
import { autoCheckForUpdates } from '../lib/update'
import { applyTheme } from '../lib/theme'
import { PROJECT_RELEASES_URL } from '../lib/version'
import { fetchAnnouncement, isAnnouncementRead, type AnnouncementData } from '../lib/announcement'
import AnnouncementModal from './AnnouncementModal'
import appLogo from '../assets/app-logo-circle.png'

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
  const [examCodeReq, setExamCodeReq] = useState<XXTExamCodeRequest | null>(null)
  const [examCodeReqs, setExamCodeReqs] = useState<XXTExamCodeRequest[]>([])
  const [snoozedExamCodeReqs, setSnoozedExamCodeReqs] = useState<Record<string, number>>({})
  const [examCode, setExamCode] = useState('')
  const [examCodeBusy, setExamCodeBusy] = useState(false)
  const [examCodeError, setExamCodeError] = useState('')

  useEffect(() => {
    applyTheme(localStorage.getItem('yatori-theme') || 'dark')
    api.getConfig().then(r => {
      if (r.ok) applyTheme(r.data.setting.basicSetting.theme || 'dark')
    }).catch(() => applyTheme(localStorage.getItem('yatori-theme') || 'dark'))
    autoCheckForUpdates().then(info => {
      if (info) setUpdateInfo(info)
    })
    const openAnnouncement = (respectRead: boolean) => {
      fetchAnnouncement().then(data => {
        if (data && (!respectRead || !isAnnouncementRead(data.id))) setAnnouncement(data)
      })
    }
    const handleOpenAnnouncement = () => openAnnouncement(false)
    openAnnouncement(true)
    window.addEventListener('yatori:open-announcement', handleOpenAnnouncement)
    return () => window.removeEventListener('yatori:open-announcement', handleOpenAnnouncement)
  }, [])

  useEffect(() => {
    const tick = () => {
      api.listXXTExamCodeRequests().then(r => {
        if (!r.ok) return
        const pending = r.data ?? []
        const now = Date.now()
        setExamCodeReqs(pending)
        setSnoozedExamCodeReqs(prev => {
          const next: Record<string, number> = {}
          Object.entries(prev).forEach(([id, until]) => {
            if (until > now && pending.some(item => item.id === id)) next[id] = until
          })
          return next
        })
        if (examCodeReq && !pending.some(item => item.id === examCodeReq.id)) {
          setExamCodeReq(null)
          setExamCode('')
          setExamCodeError('')
          return
        }
        if (!examCodeReq && pending.length) {
          const available = pending.find(item => !snoozedExamCodeReqs[item.id] || snoozedExamCodeReqs[item.id] <= now) ?? pending[0]
          setExamCodeReq(available)
          setExamCode('')
          setExamCodeError('')
        }
      }).catch(() => {})
    }
    tick()
    const timer = window.setInterval(tick, 1000)
    return () => window.clearInterval(timer)
  }, [examCodeReq, snoozedExamCodeReqs])

  const openUpdate = () => {
    api.openURL(updateInfo?.url || PROJECT_RELEASES_URL)
    setUpdateInfo(null)
  }

  const submitExamCode = async () => {
    if (!examCodeReq) return
    const code = examCode.trim()
    if (!code) {
      setExamCodeError('请输入考试码')
      return
    }
    setExamCodeBusy(true)
    const r = await api.answerXXTExamCodeRequest(examCodeReq.id, code)
    setExamCodeBusy(false)
    if (!r.ok) {
      setExamCodeError(r.error ?? '提交考试码失败')
      return
    }
    setExamCodeReq(null)
    setExamCode('')
  }

  const cancelExamCode = async () => {
    if (examCodeReq) await api.cancelXXTExamCodeRequest(examCodeReq.id).catch(() => {})
    setExamCodeReq(null)
    setExamCode('')
  }

  const snoozeExamCode = () => {
    if (!examCodeReq) return
    const now = Date.now()
    const nextSnoozed = { ...snoozedExamCodeReqs, [examCodeReq.id]: now + 30000 }
    setSnoozedExamCodeReqs(nextSnoozed)
    const next = examCodeReqs.find(item => item.id !== examCodeReq.id && (!nextSnoozed[item.id] || nextSnoozed[item.id] <= now))
    setExamCodeReq(next ?? null)
    setExamCode('')
    setExamCodeError('')
  }

  const examCodeReqIndex = examCodeReq ? examCodeReqs.findIndex(item => item.id === examCodeReq.id) + 1 : 0
  const examCodeReqCount = examCodeReqs.length

  return (
    <div className="layout">
      <nav className="nav">
        <div className="nav-logo">
          <img className="nav-logo-avatar" src={appLogo} alt="" aria-hidden="true" />
          <div className="nav-logo-copy">
            <div className="nav-logo-title">{'Yatori'}</div>
            <small>{'学习管理工具'}</small>
          </div>
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

      {examCodeReq && (
        <div className="modal-overlay">
          <div className="modal" style={{ maxWidth: 460 }}>
            <div className="modal-title">
              检测到考试需要考试码
              {examCodeReqCount > 1 && (
                <span className="badge badge-config" style={{ marginLeft: 8 }}>
                  {examCodeReqIndex || 1}/{examCodeReqCount}
                </span>
              )}
            </div>
            <div className="alert alert-info" style={{ marginBottom: 12 }}>
              仅当前账号任务暂停等待输入，其他账号会继续运行。提交后该账号会继续进入考试。
            </div>
            <div className="form-row">
              <div className="form-group">
                <label className="form-label">账号</label>
                <input className="form-input" value={examCodeReq.account || examCodeReq.uid || '—'} readOnly />
              </div>
              <div className="form-group">
                <label className="form-label">考试</label>
                <input className="form-input" value={examCodeReq.examName || '未命名考试'} readOnly />
              </div>
              <div className="form-group">
                <label className="form-label">考试码</label>
                <input
                  className="form-input"
                  autoFocus
                  value={examCode}
                  placeholder="请输入本次考试码"
                  onChange={e => setExamCode(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === 'Enter') submitExamCode()
                  }}
                />
              </div>
            </div>
            {examCodeError && <div className="alert alert-warn" style={{ marginTop: 10 }}>{examCodeError}</div>}
            <div className="modal-footer">
              <button className="btn btn-ghost" onClick={cancelExamCode} disabled={examCodeBusy}>取消</button>
              {examCodeReqCount > 1 && (
                <button className="btn btn-ghost" onClick={snoozeExamCode} disabled={examCodeBusy}>稍后处理</button>
              )}
              <button className="btn btn-primary" onClick={submitExamCode} disabled={examCodeBusy}>
                {examCodeBusy ? '提交中…' : '提交并继续'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
