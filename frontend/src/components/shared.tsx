import { useState, useEffect, useRef, ReactNode } from 'react'
import { api, onTaskLog } from '../lib/api'

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
      <div className="modal" style={{ minWidth: 340, maxWidth: 420 }}>
        <div className="modal-title">确认操作</div>
        <p style={{ color: 'var(--text2)', lineHeight: 1.6 }}>{msg}</p>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onCancel}>取消</button>
          <button className="btn btn-danger" onClick={onOk}>确认</button>
        </div>
      </div>
    </div>
  )
}

export function Spinner({ text = '加载中…' }: { text?: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '24px 0', color: 'var(--text2)', fontSize: 13 }}>
      <span style={{
        width: 16, height: 16,
        border: '2px solid var(--border2)',
        borderTopColor: 'var(--accent)',
        borderRadius: '50%',
        display: 'inline-block',
        animation: 'spin .7s linear infinite',
        flexShrink: 0,
      }} />
      {text}
    </div>
  )
}

export function Section({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <div className="card" style={{ marginBottom: 14 }}>
      <div className="section-header">
        <div className="card-title" style={{ marginBottom: 0 }}>{title}</div>
        {action}
      </div>
      {children}
    </div>
  )
}

export function FormGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="form-group">
      {label && <label className="form-label">{label}</label>}
      {children}
    </div>
  )
}

export function EmptyState({ text }: { text: string }) {
  return (
    <div style={{ textAlign: 'center', padding: '40px 24px', color: 'var(--text3)', fontSize: 13 }}>
      {text}
    </div>
  )
}
