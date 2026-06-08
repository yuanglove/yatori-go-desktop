import { useState, useEffect, useRef } from 'react'
import { api, onAnyLog } from '../lib/api'
import type { AccountVO, TaskStatus } from '../lib/api'

type UIState = 'idle' | 'running' | 'stopping' | 'stopped' | 'failed'

function stateBadge(s: UIState) {
  if (s === 'running')  return <span className="badge badge-running">运行中</span>
  if (s === 'stopping') return <span className="badge badge-config">停止中…</span>
  if (s === 'failed')   return <span className="badge badge-failed">失败</span>
  if (s === 'stopped')  return <span className="badge badge-stopped">已停止</span>
  return <span className="badge badge-stopped">未启动</span>
}

export default function TasksPage() {
  const [accounts, setAccounts] = useState<AccountVO[]>([])
  const [statuses, setStatuses] = useState<Map<string, TaskStatus>>(new Map())
  const [uiState, setUIState] = useState<Map<string, UIState>>(new Map())
  const [loading, setLoading] = useState(true)
  const [lastLogs, setLastLogs] = useState<Map<string, string>>(new Map())
  const stoppingAt = useRef<Map<string, number>>(new Map())
  const STOP_TIMEOUT_MS = 15_000

  const loadAll = async () => {
    const [ar, sr] = await Promise.all([api.listAccounts(), api.getTaskStatuses()])
    if (ar.ok) setAccounts(ar.data)
    if (sr.ok) {
      const m = new Map<string, TaskStatus>()
      sr.data.forEach(s => m.set(s.uid, s))
      setStatuses(m)
      const now = Date.now()
      setUIState(prev => {
        const next = new Map(prev)
        sr.data.forEach(s => {
          const cur = prev.get(s.uid)
          if (cur === 'stopping') {
            const t = stoppingAt.current.get(s.uid) ?? 0
            if (s.state !== 'running' || now - t > STOP_TIMEOUT_MS) {
              next.set(s.uid, s.state as UIState)
              stoppingAt.current.delete(s.uid)
            }
          } else {
            next.set(s.uid, s.state as UIState)
          }
        })
        return next
      })
    }
    setLoading(false)
  }

  useEffect(() => {
    loadAll()
    const t = setInterval(loadAll, 4000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    const off = onAnyLog(item => {
      setLastLogs(prev => new Map(prev).set(item.uid, item.msg))
      if (/\]\s*\[错误\]/.test(item.msg) || /\]\s*ERROR\b/.test(item.msg) || /^\s*(ERROR|FATAL)\b/.test(item.msg)) {
        setUIState(prev => new Map(prev).set(item.uid, 'failed'))
        stoppingAt.current.delete(item.uid)
      }
    })
    return off
  }, [])

  const start = async (uid: string) => {
    setUIState(prev => new Map(prev).set(uid, 'running'))
    const r = await api.startTask(uid)
    if (!r.ok) {
      setUIState(prev => new Map(prev).set(uid, 'failed'))
      setLastLogs(prev => new Map(prev).set(uid, r.error ?? '启动失败'))
    }
  }

  const stop = async (uid: string) => {
    stoppingAt.current.set(uid, Date.now())
    setUIState(prev => new Map(prev).set(uid, 'stopping'))
    await api.stopTask(uid)
  }

  if (loading) return <div className="page"><span className="text-muted" style={{ fontSize: 13 }}>加载中…</span></div>

  return (
    <div className="page">
      <div className="page-title">任务控制</div>

      <div className="alert alert-info" style={{ marginBottom: 16 }}>
        学习通 GUI 模式支持：登录、启动、停止、课程过滤、普通/多课程/多任务点模式。
        CxNode 控制同一账号内同时进行的视频任务点数量；全局最大任务数控制同时运行的账号数量。
      </div>

      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        <table className="table">
          <thead>
            <tr>
              <th>账号</th>
              <th>平台</th>
              <th>状态</th>
              <th>最近日志</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {accounts.map(a => {
              const sv = statuses.get(a.uid)
              const ui: UIState = uiState.get(a.uid) ?? 'idle'
              const log = lastLogs.get(a.uid) ?? sv?.lastLog ?? '—'
              const noCtrl = a.guiSupport !== 'full'
              const isRunning = ui === 'running'
              const isStopping = ui === 'stopping'
              return (
                <tr key={a.uid}>
                  <td>
                    <div style={{ fontWeight: 500 }}>{a.remarkName || a.account}</div>
                    {a.remarkName && <div className="text-muted text-sm">{a.account}</div>}
                  </td>
                  <td><span className="badge badge-config">{a.accountType}</span></td>
                  <td>{stateBadge(ui)}</td>
                  <td className="text-muted text-sm" style={{ maxWidth: 240, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontFamily: 'var(--font-mono, monospace)' }}>
                    {log}
                  </td>
                  <td>
                    <div className="flex-row">
                      {!noCtrl && !isRunning && !isStopping && (
                        <button className="btn btn-primary btn-sm" onClick={() => start(a.uid)}>启动</button>
                      )}
                      {noCtrl && !isRunning && !isStopping && (
                        <span className="text-muted text-sm" title={`${a.accountType} 暂不支持单账号 GUI 控制`}>暂不支持</span>
                      )}
                      {(isRunning || isStopping) && (
                        <button className="btn btn-danger btn-sm"
                          disabled={isStopping}
                          onClick={() => stop(a.uid)}>
                          {isStopping ? '停止中…' : '停止'}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              )
            })}
            {accounts.length === 0 && (
              <tr>
                <td colSpan={5} style={{ textAlign: 'center', padding: '32px 24px', color: 'var(--text3)' }}>
                  请先在"账号管理"添加账号
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
