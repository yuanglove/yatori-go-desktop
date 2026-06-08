import { useEffect, useState } from 'react'
import { Users, Play, FileCheck, FolderOpen, RefreshCw } from 'lucide-react'
import { api } from '../lib/api'
import type { Dashboard } from '../lib/api'
import { Spinner } from '../components/shared'

export default function DashboardPage() {
  const [data, setData] = useState<Dashboard | null>(null)
  const [loading, setLoading] = useState(true)

  const load = () => {
    setLoading(true)
    api.getDashboard().then(r => {
      if (r.ok) setData(r.data)
    }).finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    const t = setInterval(load, 10000)
    return () => clearInterval(t)
  }, [])

  if (loading && !data) return <div className="page"><Spinner /></div>

  return (
    <div className="page">
      <div className="page-title">{'仪表盘'}</div>

      <div className="stats-row">
        <div className="stat-card">
          <span className="stat-icon"><Users size={18} strokeWidth={1.75} /></span>
          <div className="stat-value">{data?.totalAccounts ?? 0}</div>
          <div className="stat-label">{'账号总数'}</div>
        </div>
        <div className="stat-card">
          <span className="stat-icon"><Play size={18} strokeWidth={1.75} /></span>
          <div className="stat-value" style={{ color: 'var(--success)' }}>{data?.runningTasks ?? 0}</div>
          <div className="stat-label">{'运行中任务'}</div>
        </div>
        <div className="stat-card">
          <span className="stat-icon"><FileCheck size={18} strokeWidth={1.75} /></span>
          <div className="stat-value" style={{ fontSize: 18, paddingTop: 2 }}>
            <span className={data?.configOK ? 'badge badge-running' : 'badge badge-failed'}>
              {data?.configOK ? '正常' : '未找到'}
            </span>
          </div>
          <div className="stat-label">{'配置文件'}</div>
        </div>
        <div className="stat-card">
          <span className="stat-icon"><FolderOpen size={18} strokeWidth={1.75} /></span>
          <div className="stat-value" style={{ fontSize: 11, fontWeight: 500, color: 'var(--text2)', letterSpacing: 0, lineHeight: 1.4, wordBreak: 'break-all' }}>
            {data?.configPath ?? '-'}
          </div>
          <div className="stat-label">{'数据目录'}</div>
        </div>
      </div>

      <div className="card">
        <div className="flex-between" style={{ marginBottom: 10 }}>
          <div className="card-title" style={{ marginBottom: 0 }}>{'最近日志'}</div>
          <button className="btn btn-ghost btn-sm" onClick={load}>
            <RefreshCw size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            {'刷新'}
          </button>
        </div>
        <div className="log-area">
          {(data?.recentLogs ?? []).length === 0
            ? <span className="text-muted">{'暂无日志'}</span>
            : data!.recentLogs.map((l, i) => <div key={i} className="log-line log-INFO">{l}</div>)
          }
        </div>
      </div>
    </div>
  )
}
