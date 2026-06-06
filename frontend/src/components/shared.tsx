import { useState, useEffect, useRef, ReactNode } from 'react'
import { api, onTaskLog } from '../lib/api'

// 匹配所有 DTO 的公共形状（ok + data? + error?）
interface AnyResult<T = unknown> { ok: boolean; data?: T; error?: string }

export function useAsync<T>(fn: () => Promise<AnyResult<T>>, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const load = () => {
    setLoading(true)
    fn().then(r => { r.ok ? setData(r.data as T) : setError(r.error ?? '请求失败') })
       .catch(e => setError(String(e)))
       .finally(() => setLoading(false))
  }
  useEffect(load, deps)
  return { data, loading, error, reload: load }
}

export function useLogStream(uid: string) {
  const [lines, setLines] = useState<string[]>([])
  const ref = useRef<HTMLDivElement>(null)
  const paused = useRef(false)
  useEffect(() => {
    api.tailLog(200).then(r => { if (r.ok) setLines(r.data) })
    const off = onTaskLog(uid, (msg: string) => {
      setLines(prev => [...prev.slice(-500), msg])
      if (!paused.current && ref.current)
        ref.current.scrollTop = ref.current.scrollHeight
    })
    return off
  }, [uid])
  return { lines, ref, setPaused: (v: boolean) => { paused.current = v } }
}

export function Confirm({ msg, onOk, onCancel }: { msg: string; onOk(): void; onCancel(): void }) {
  return (
    <div className="modal-overlay">
      <div className="modal" style={{ minWidth: 320 }}>
        <div className="modal-title">确认</div>
        <p style={{ marginBottom: 8 }}>{msg}</p>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onCancel}>取消</button>
          <button className="btn btn-danger" onClick={onOk}>确认</button>
        </div>
      </div>
    </div>
  )
}

export function Spinner() {
  return <span style={{ opacity: .6, fontSize: 12 }}>加载中…</span>
}

export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="card">
      <div className="card-title">{title}</div>
      {children}
    </div>
  )
}

export function FormGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="form-group">
      <label className="form-label">{label}</label>
      {children}
    </div>
  )
}
