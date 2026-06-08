import { useState, useEffect, useRef } from 'react'
import { api, onAnyLog } from '../lib/api'

export default function LogsPage() {
  const [lines, setLines] = useState<string[]>([])
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    // 加载时先拉内存缓冲 + 文件历史
    api.getRecentLogs(300).then(r => { if (r.ok && r.data.length) setLines(r.data) })
    // 订阅后续实时事件
    const stripANSI = (s: string) => s.replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
    const off = onAnyLog(item => {
      setLines(prev => [...prev.slice(-600), stripANSI(item.msg)])
    })
    return off
  }, [])

  useEffect(() => {
    if (!paused && ref.current) ref.current.scrollTop = ref.current.scrollHeight
  }, [lines, paused])

  const visible = filter ? lines.filter(l => l.toLowerCase().includes(filter.toLowerCase())) : lines

  const isErrorLog = (l: string) =>
    /\]\s*\[错误\]/.test(l) ||
    /\]\s*ERROR\b/.test(l) ||
    /^\s*(ERROR|FATAL)\b/.test(l)

  const cls = (l: string) => {
    if (isErrorLog(l)) return 'log-line log-ERROR'
    if (/\]\s*WARN\b/.test(l) || /^\s*WARN\b/.test(l)) return 'log-line log-WARN'
    if (/\]\s*DEBUG\b/.test(l) || /^\s*DEBUG\b/.test(l)) return 'log-line log-DEBUG'
    return 'log-line log-INFO'
  }

  return (
    <div className="page">
      <div className="flex-between" style={{ marginBottom: 12 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>日志中心</div>
        <div className="flex-row">
          <input className="form-input" style={{ width: 200 }} placeholder="关键词筛选…"
            value={filter} onChange={e => setFilter(e.target.value)} />
          <button className="btn btn-ghost btn-sm" onClick={() => setPaused(p => !p)}>
            {paused ? '▶ 恢复' : '⏸ 暂停'}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => setLines([])}>清空界面</button>
          <button className="btn btn-ghost btn-sm"
            onClick={() => api.getRecentLogs(300).then(r => r.ok && setLines(r.data))}>
            刷新
          </button>
        </div>
      </div>
      <div className="log-area" ref={ref} style={{ height: 'calc(100vh - 160px)' }}>
        {visible.length === 0
          ? <span className="text-muted">暂无日志（启动任务后将在此显示）</span>
          : visible.map((l, i) => <div key={i} className={cls(l)}>{l}</div>)
        }
      </div>
      <div className="text-muted text-sm" style={{ marginTop: 6 }}>
        {visible.length} 行{filter && ` · 过滤中（原 ${lines.length} 行）`} · 清空界面不影响文件日志
      </div>
    </div>
  )
}
