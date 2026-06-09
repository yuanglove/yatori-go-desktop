import { useState, useEffect, useRef } from 'react'
import { Pause, Play, Trash2, RefreshCw } from 'lucide-react'
import { api, onAnyLog } from '../lib/api'
import AnimatedSelect from '../components/AnimatedSelect'

type LogLevel = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'

const LOG_LEVELS: LogLevel[] = ['DEBUG', 'INFO', 'WARN', 'ERROR']
const LOG_LEVEL_STORAGE_KEY = 'yatori-log-center-level'
const LEVEL_WEIGHT: Record<LogLevel, number> = {
  DEBUG: 0,
  INFO: 1,
  WARN: 2,
  ERROR: 3,
}

function detectLevel(line: string): LogLevel {
  if (/\]\s*\[错误\]/.test(line) || /\]\s*ERROR\b/.test(line) || /^\s*(ERROR|FATAL)\b/.test(line)) return 'ERROR'
  if (/\]\s*WARN\b/.test(line) || /^\s*WARN\b/.test(line) || /警告|WARN/.test(line)) return 'WARN'
  if (/\]\s*DEBUG\b/.test(line) || /^\s*DEBUG\b/.test(line) || /调试|DEBUG/.test(line)) return 'DEBUG'
  return 'INFO'
}

function stripANSI(s: string) {
  return s.replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '')
}

export default function LogsPage() {
  const [lines, setLines] = useState<string[]>([])
  const [paused, setPaused] = useState(false)
  const [filter, setFilter] = useState('')
  const [level, setLevel] = useState<LogLevel>(() => {
    const saved = localStorage.getItem(LOG_LEVEL_STORAGE_KEY)?.toUpperCase()
    return LOG_LEVELS.includes(saved as LogLevel) ? saved as LogLevel : 'INFO'
  })
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!localStorage.getItem(LOG_LEVEL_STORAGE_KEY)) {
      api.getConfig().then(r => {
        const cfgLevel = r.ok ? String(r.data?.setting.basicSetting.logLevel || 'INFO').toUpperCase() : 'INFO'
        if (LOG_LEVELS.includes(cfgLevel as LogLevel)) setLevel(cfgLevel as LogLevel)
      })
    }
    api.getRecentLogs(300).then(r => { if (r.ok && r.data.length) setLines(r.data.map(stripANSI)) })
    const off = onAnyLog(item => {
      setLines(prev => [...prev.slice(-600), stripANSI(item.msg)])
    })
    return off
  }, [])

  const setLogLevel = (next: LogLevel) => {
    setLevel(next)
    localStorage.setItem(LOG_LEVEL_STORAGE_KEY, next)
  }

  useEffect(() => {
    if (!paused && ref.current) ref.current.scrollTop = ref.current.scrollHeight
  }, [lines, paused])

  const visible = lines.filter(line => {
    if (LEVEL_WEIGHT[detectLevel(line)] < LEVEL_WEIGHT[level]) return false
    if (filter && !line.toLowerCase().includes(filter.toLowerCase())) return false
    return true
  })

  const cls = (line: string) => `log-line log-${detectLevel(line)}`

  const reload = async () => {
    const r = await api.getRecentLogs(300)
    if (r.ok) setLines(r.data.map(stripANSI))
  }

  const statusText = () => {
    const parts = [`${visible.length} 行`, `等级 >= ${level}`]
    if (filter) parts.push(`关键词过滤中（原 ${lines.length} 行）`)
    parts.push('清空界面不影响文件日志')
    return parts.join(' · ')
  }

  return (
    <div className="page">
      <div className="flex-between" style={{ marginBottom: 14 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>日志中心</div>
        <div className="flex-row">
          <div style={{ width: 110 }}>
            <AnimatedSelect
              value={level}
              options={LOG_LEVELS.map(item => ({ value: item, label: item }))}
              onChange={v => setLogLevel(String(v) as LogLevel)}
              ariaLabel="日志等级过滤"
            />
          </div>
          <input
            className="form-input"
            style={{ width: 200 }}
            placeholder="关键词筛选"
            value={filter}
            onChange={e => setFilter(e.target.value)}
          />
          <button className="btn btn-ghost btn-sm" onClick={() => setPaused(p => !p)}>
            {paused
              ? <><Play size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />恢复</>
              : <><Pause size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />暂停</>
            }
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => setLines([])}>
            <Trash2 size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            清空
          </button>
          <button className="btn btn-ghost btn-sm" onClick={reload}>
            <RefreshCw size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
            刷新
          </button>
        </div>
      </div>

      <div className="log-area" ref={ref} style={{ height: 'calc(100vh - 148px)' }}>
        {visible.length === 0
          ? <span className="text-muted">暂无匹配日志。切到 DEBUG 可查看全部等级日志。</span>
          : visible.map((line, i) => <div key={i} className={cls(line)}>{line}</div>)
        }
      </div>

      <div className="text-muted text-sm" style={{ marginTop: 6 }}>
        {statusText()}
      </div>
    </div>
  )
}
