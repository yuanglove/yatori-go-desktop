import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import type { CourseVO, AccountVO } from '../lib/api'
import { Spinner, EmptyState } from '../components/shared'
import AnimatedSelect from '../components/AnimatedSelect'

const SUPPORTED = new Set(['XUEXITONG', 'YINGHUA', 'HQKJ', 'WELEARN', 'ICVE'])

function isCourseComplete(c: CourseVO): boolean {
  if (c.jobCount > 0 && c.jobFinishCount >= c.jobCount) return true
  return c.hasProgress && c.jobRate >= 100
}

function getCourseTaskText(c: CourseVO): string {
  if (c.platform === 'ICVE') {
    if (c.jobCount > 0) return `${c.jobFinishCount}/${c.jobCount}`
    return '课程已获取，任务点待同步'
  }
  if (c.jobCount > 0) {
    return `${c.jobFinishCount}/${c.jobCount}`
  }
  return c.hasProgress ? '未返回' : (c.rawStatusText || '—')
}

function getCourseProgressText(c: CourseVO): string {
  if (c.platform === 'ICVE' && c.jobCount === 0) {
    return c.rawStatusText ? '待同步' : '暂无进度'
  }
  return c.hasProgress ? `${c.jobRate.toFixed(0)}%` : '暂无进度'
}

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
    if (!selectedUid || loading) return
    setLoading(true); setErr('')
    try {
      const r = await api.getCourses(selectedUid).catch(e => ({ ok: false, data: [] as CourseVO[], error: String(e) }))
      if (r.ok) setCourses(r.data ?? [])
      else setErr(r.error ?? '请求失败')
    } finally {
      setLoading(false)
    }
  }

  const filtered = courses.filter(c => {
    if (search && !c.courseName.includes(search)) return false
    if (onlyIncomplete && isCourseComplete(c)) return false
    return true
  })

  return (
    <div className="page">
      <div className="page-title">课程进度</div>

      <div className="card">
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ width: 300, maxWidth: '100%' }}>
            <AnimatedSelect
            value={selectedUid}
              options={accounts.map((a: AccountVO) => ({
                value: a.uid,
                label: `[${a.accountType}] ${a.remarkName || a.account}`,
              }))}
              placeholder="请选择账号"
              onChange={v => { setUid(String(v)); setCourses([]); setErr('') }}
            />
          </div>
          <button className="btn btn-primary" onClick={load} disabled={loading || !selectedUid}>
            {loading ? '加载中…' : '刷新课程进度'}
          </button>
          {courses.length > 0 && (
            <>
              <input
                className="form-input"
                style={{ maxWidth: 200 }}
                placeholder="搜索课程名…"
                value={search}
                onChange={e => setSearch(e.target.value)}
              />
              <label style={{ display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer', fontSize: 13, color: 'var(--text2)' }}>
                <input type="checkbox" checked={onlyIncomplete} onChange={e => setOnlyIncomplete(e.target.checked)} />
                只看未完成
              </label>
              <span className="text-muted text-sm">{filtered.length} / {courses.length} 门</span>
            </>
          )}
        </div>
        {selectedAccount && !supported && (
          <div style={{ marginTop: 10, fontSize: 13, color: 'var(--warn)' }}>
            暂不支持 {selectedAccount.accountType} 平台课程进度拉取
          </div>
        )}
      </div>

      {err && <div className="alert alert-warn">{err}</div>}

      {loading && <div className="card"><Spinner /></div>}

      {!loading && courses.length === 0 && !err && (
        <div className="card">
          <EmptyState text="暂无课程数据，请选择支持的平台账号后点击刷新" />
        </div>
      )}

      {!loading && filtered.length === 0 && courses.length > 0 && (
        <div className="card">
          <EmptyState text="无匹配课程" />
        </div>
      )}

      {filtered.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <table className="table">
            <thead>
              <tr>
                <th>平台</th>
                <th style={{ minWidth: 210 }}>课程名称</th>
                <th>任课老师</th>
                <th>任务</th>
                <th>状态</th>
                <th style={{ minWidth: 160 }}>进度</th>
                <th>课程 ID</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((c, i) => (
                <tr key={c.key || c.courseId || i}>
                  <td><span className="badge badge-config" style={{ fontSize: 10 }}>{c.platform || '—'}</span></td>
                  <td style={{ fontWeight: 500 }}>{c.courseName || '—'}</td>
                  <td className="text-muted">{c.courseTeacher || '—'}</td>
                  <td className="text-muted">{getCourseTaskText(c)}</td>
                  <td>
                    {isCourseComplete(c)
                      ? <span className="badge badge-stopped">已完成</span>
                      : !c.isStart
                      ? <span className="badge badge-stopped">未开课</span>
                      : c.platform === 'ICVE' && c.jobCount === 0
                        ? <span className="badge badge-pending">待同步</span>
                      : c.state === 1
                        ? <span className="badge badge-stopped">已结课</span>
                        : <span className="badge badge-running">进行中</span>}
                  </td>
                  <td>
                    {c.platform === 'ICVE' && c.jobCount === 0 ? (
                      <span className="text-muted text-sm">{getCourseProgressText(c)}</span>
                    ) : c.hasProgress ? (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <div className="progress-bar-track">
                          <div
                            className={`progress-bar-fill${c.jobRate >= 100 ? ' complete' : ''}`}
                            style={{ width: `${Math.min(c.jobRate, 100)}%` }}
                          />
                        </div>
                        <span style={{
                          minWidth: 36,
                          textAlign: 'right',
                          fontSize: 12,
                          fontWeight: 600,
                          color: c.jobRate >= 100 ? 'var(--success)' : 'var(--text2)',
                        }}>
                          {getCourseProgressText(c)}
                        </span>
                      </div>
                    ) : <span className="text-muted text-sm">暂无进度</span>}
                  </td>
                  <td className="text-muted text-sm">{c.courseId || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
