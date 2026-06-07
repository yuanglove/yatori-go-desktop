import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import type { CourseVO, AccountVO } from '../lib/api'
import { Spinner } from '../components/shared'

const SUPPORTED = new Set(['XUEXITONG', 'YINGHUA', 'HQKJ', 'WELEARN'])

export default function CoursesPage() {
  const [accounts, setAccounts] = useState<AccountVO[]>([])
  const [uid, setUid] = useState<string>('')
  const [courses, setCourses] = useState<CourseVO[]>([])
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState('')
  const [search, setSearch] = useState('')
  const [onlyIncomplete, setOnlyIncomplete] = useState(false)

  useEffect(() => {
    api.listAccounts().then(r => { if (r.ok) setAccounts(r.data ?? []) })
  }, [])

  const selectedUid = uid || accounts[0]?.uid || ''
  const selectedAccount = accounts.find(a => a.uid === selectedUid)
  const supported = selectedAccount ? SUPPORTED.has(selectedAccount.accountType) : false

  const load = async () => {
    if (!selectedUid) return
    setLoading(true); setErr('')
    const r = await api.getCourses(selectedUid).catch(e => ({ ok: false, data: [] as CourseVO[], error: String(e) }))
    if (r.ok) setCourses(r.data ?? [])
    else setErr(r.error ?? '请求失败')
    setLoading(false)
  }

  const filtered = courses.filter(c => {
    if (search && !c.courseName.includes(search)) return false
    if (onlyIncomplete && c.jobRate >= 100) return false
    return true
  })

  return (
    <div className="page">
      <div className="page-title">课程进度</div>

      <div className="card" style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <select
            className="form-input"
            style={{ maxWidth: 280 }}
            value={selectedUid}
            onChange={e => { setUid(e.target.value); setCourses([]); setErr('') }}
          >
            {accounts.map((a: AccountVO) => (
              <option key={a.uid} value={a.uid}>
                [{a.accountType}] {a.remarkName || a.account}
              </option>
            ))}
          </select>
          <button className="btn btn-primary" onClick={load} disabled={loading || !selectedUid}>
            {loading ? '加载中...' : '刷新课程进度'}
          </button>
        </div>
        {selectedAccount && !supported && (
          <div style={{ marginTop: 12, color: 'var(--warning, #e6a817)' }}>
            暂不支持 {selectedAccount.accountType} 平台课程进度拉取
          </div>
        )}
      </div>

      {err && <div className="card" style={{ color: 'var(--danger, #e05252)', marginBottom: 16 }}>{err}</div>}

      {courses.length > 0 && (
        <div className="card" style={{ marginBottom: 12 }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              className="form-input"
              style={{ maxWidth: 240 }}
              placeholder="搜索课程名..."
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
            <label style={{ display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer' }}>
              <input type="checkbox" checked={onlyIncomplete} onChange={e => setOnlyIncomplete(e.target.checked)} />
              只看未完成
            </label>
            <span style={{ color: 'var(--text-muted)', fontSize: 13 }}>
              共 {filtered.length} / {courses.length} 门
            </span>
          </div>
        </div>
      )}

      {loading && <div className="card"><Spinner /></div>}

      {!loading && courses.length === 0 && !err && (
        <div className="card" style={{ color: 'var(--text-muted)', textAlign: 'center', padding: 32 }}>
          暂无课程数据，请选择支持的平台账号后点击刷新
        </div>
      )}

      {!loading && filtered.length === 0 && courses.length > 0 && (
        <div className="card" style={{ color: 'var(--text-muted)', textAlign: 'center', padding: 24 }}>
          无匹配课程
        </div>
      )}

      {filtered.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: 'var(--bg-secondary, rgba(0,0,0,.15))' }}>
                <th style={th}>平台</th>
                <th style={th}>课程名称</th>
                <th style={th}>任课老师</th>
                <th style={th}>任务</th>
                <th style={th}>状态</th>
                <th style={{ ...th, minWidth: 160 }}>进度</th>
                <th style={th}>课程ID / classId</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((c, i) => (
                <tr key={c.key || c.courseId || i} style={{ borderTop: '1px solid var(--border)' }}>
                  <td style={{ ...td, fontSize: 11 }}><span className="badge badge-config">{c.platform || '—'}</span></td>
                  <td style={td}>{c.courseName || '—'}</td>
                  <td style={td}>{c.courseTeacher || '—'}</td>
                  <td style={td}>
                    {c.jobCount > 0 ? `${c.jobFinishCount}/${c.jobCount}` : (c.rawStatusText || '—')}
                  </td>
                  <td style={td}>
                    {!c.isStart
                      ? <span style={{ color: 'var(--text-muted)' }}>未开课</span>
                      : c.state === 1
                        ? <span style={{ color: 'var(--text-muted)' }}>已结课</span>
                        : <span style={{ color: 'var(--success, #4caf50)' }}>进行中</span>}
                  </td>
                  <td style={td}>
                    {c.jobCount > 0 ? (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <div style={{ flex: 1, height: 6, background: 'var(--bg-secondary, rgba(0,0,0,.2))', borderRadius: 3, overflow: 'hidden' }}>
                          <div style={{
                            height: '100%',
                            width: `${Math.min(c.jobRate, 100)}%`,
                            background: c.jobRate >= 100 ? 'var(--success, #4caf50)' : 'var(--primary, #5b8dee)',
                            borderRadius: 3,
                          }} />
                        </div>
                        <span style={{ minWidth: 36, textAlign: 'right', color: c.jobRate >= 100 ? 'var(--success, #4caf50)' : undefined }}>
                          {c.jobRate.toFixed(0)}%
                        </span>
                      </div>
                    ) : <span style={{ color: 'var(--text-muted)' }}>暂无进度数据</span>}
                  </td>
                  <td style={{ ...td, color: 'var(--text-muted)', fontSize: 11 }}>
                    {c.courseId || '—'} / {c.key || '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

const th: React.CSSProperties = { padding: '8px 12px', textAlign: 'left', fontWeight: 600, whiteSpace: 'nowrap' }
const td: React.CSSProperties = { padding: '10px 12px', verticalAlign: 'middle' }
