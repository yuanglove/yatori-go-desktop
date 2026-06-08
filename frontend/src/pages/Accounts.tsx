import { useState } from 'react'
import { api } from '../lib/api'
import type { AccountVO, AccountReq, CoursesCustom } from '../lib/api'
import { useAsync, Confirm, Section, FormGroup, Spinner } from '../components/shared'

const PLATFORMS = [
  { code: 'YINGHUA', name: '英华学堂' }, { code: 'XUEXITONG', name: '学习通' },
  { code: 'ENAEA', name: '学习公社' }, { code: 'CQIE', name: '重庆工程学院' },
  { code: 'KETANGX', name: '码上研训' }, { code: 'WELEARN', name: 'WeLearn 随行课堂' },
  { code: 'ICVE', name: '智慧职教' }, { code: 'QSXT', name: '青书学堂' },
  { code: 'HQKJ', name: '海旗科技' }, { code: 'CANGHUI', name: '仓辉实训' },
]

const emptyReq = (): AccountReq => ({
  uid: '', accountType: 'YINGHUA', url: '', remarkName: '', account: '',
  password: '', isProxy: 0, informEmails: [], coursesCustom: {
    shuffleSw: 0, videoModel: 1, autoExam: 0, examAutoSubmit: 1,
    includeCourses: [], excludeCourses: [],
  },
})

function guiBadge(g: string) {
  if (g === 'full') return <span className="badge badge-full">完整支持</span>
  if (g === 'config-only') return <span className="badge badge-config">仅配置</span>
  return <span className="badge badge-none">暂不支持</span>
}

export default function AccountsPage() {
  const { data: accounts, loading, reload } = useAsync(api.listAccounts)
  const [editing, setEditing] = useState<AccountReq | null>(null)
  const [delUid, setDelUid] = useState('')
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')

  const openAdd = () => { setErr(''); setEditing(emptyReq()) }
  const openEdit = (a: AccountVO) => {
    setErr('')
    setEditing({ uid: a.uid, accountType: a.accountType, url: a.url, remarkName: a.remarkName ?? '',
      account: a.account, password: '', isProxy: a.isProxy, informEmails: a.informEmails,
      coursesCustom: a.coursesCustom })
  }

  const handleSave = async () => {
    if (!editing) return
    if (editing.accountType === 'YINGHUA' && !editing.url.trim()) {
      setErr('英华平台必须填写学校入口地址（平台 URL）'); return
    }
    setSaving(true); setErr('')
    const r = editing.uid ? await api.updateAccount(editing) : await api.addAccount(editing)
    setSaving(false)
    if (!r.ok) { setErr(r.error ?? '操作失败'); return }
    setEditing(null); reload()
  }

  const handleDelete = async () => {
    await api.deleteAccount(delUid)
    setDelUid(''); reload()
  }

  return (
    <div className="page">
      <div className="flex-between" style={{ marginBottom: 16 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>账号管理</div>
        <button className="btn btn-primary" onClick={openAdd}>+ 添加账号</button>
      </div>
      {loading ? <Spinner /> : (
        <div className="card" style={{ padding: 0 }}>
          <table className="table">
            <thead>
              <tr>
                <th>平台</th><th>账号</th><th>备注</th><th>代理</th><th>GUI 支持</th><th>操作</th>
              </tr>
            </thead>
            <tbody>
              {(accounts ?? []).map(a => (
                <tr key={a.uid}>
                  <td><span className="badge badge-config">{a.accountType}</span></td>
                  <td>{a.account}</td>
                  <td className="text-muted">{a.remarkName || '—'}</td>
                  <td>{a.isProxy ? '是' : '否'}</td>
                  <td>{guiBadge(a.guiSupport)}</td>
                  <td>
                    <div className="flex-row">
                      <button className="btn btn-ghost btn-sm" onClick={() => openEdit(a)}>编辑</button>
                      <button className="btn btn-danger btn-sm" onClick={() => setDelUid(a.uid)}>删除</button>
                    </div>
                  </td>
                </tr>
              ))}
              {(accounts ?? []).length === 0 && (
                <tr><td colSpan={6} style={{ textAlign: 'center', padding: 24, color: 'var(--text2)' }}>
                  暂无账号，点击"添加账号"开始
                </td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {editing && <AccountModal req={editing} onChange={setEditing}
        onSave={handleSave} onClose={() => setEditing(null)} saving={saving} error={err} />}
      {delUid && <Confirm msg="确认删除该账号？运行中的任务将被停止。"
        onOk={handleDelete} onCancel={() => setDelUid('')} />}
    </div>
  )
}

function AccountModal({ req, onChange, onSave, onClose, saving, error }: {
  req: AccountReq
  onChange(r: AccountReq): void
  onSave(): void
  onClose(): void
  saving: boolean
  error: string
}) {
  const set = (k: keyof AccountReq, v: unknown) => onChange({ ...req, [k]: v })
  const setCC = (k: keyof CoursesCustom, v: unknown) =>
    onChange({ ...req, coursesCustom: { ...req.coursesCustom, [k]: v } })

  return (
    <div className="modal-overlay">
      <div className="modal">
        <div className="modal-title">{req.uid ? '编辑账号' : '添加账号'}</div>
        {error && <div className="alert alert-warn">{error}</div>}

        <Section title="基本信息">
          <div className="form-row form-row-2">
            <FormGroup label="平台类型 *">
              <select className="form-select" value={req.accountType}
                onChange={e => set('accountType', e.target.value)}>
                {PLATFORMS.map(p => <option key={p.code} value={p.code}>{p.name} ({p.code})</option>)}
              </select>
            </FormGroup>
            <FormGroup label={req.accountType === 'YINGHUA' ? '平台 URL *（英华必填）' : '平台 URL'}>
              <input className="form-input" value={req.url}
                placeholder={req.accountType === 'YINGHUA' ? '必填，如 https://xxx.yinghuaxuetang.com' : '部分平台必填'}
                onChange={e => set('url', e.target.value)} />
            </FormGroup>
          </div>
          <div className="form-row form-row-2">
            <FormGroup label="账号 *">
              <input className="form-input" value={req.account}
                onChange={e => set('account', e.target.value)} />
            </FormGroup>
            <FormGroup label={req.uid ? '密码（留空则保留原密码）' : '密码 *'}>
              <input className="form-input" type="password" value={req.password}
                placeholder={req.uid ? '已保存密码，留空不修改' : ''}
                onChange={e => set('password', e.target.value)} />
              {req.uid && <div className="text-muted text-sm" style={{marginTop:3}}>当前账号已有本地保存的密码</div>}
            </FormGroup>
          </div>
          <div className="form-row form-row-2">
            <FormGroup label="备注名">
              <input className="form-input" value={req.remarkName}
                onChange={e => set('remarkName', e.target.value)} />
            </FormGroup>
            <FormGroup label="使用代理">
              <select className="form-select" value={req.isProxy}
                onChange={e => set('isProxy', Number(e.target.value))}>
                <option value={0}>否</option><option value={1}>是</option>
              </select>
            </FormGroup>
          </div>
          <FormGroup label="通知邮箱（多个用英文逗号分隔）">
            <input className="form-input"
              value={(req.informEmails ?? []).join(',')}
              onChange={e => set('informEmails', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
          </FormGroup>
        </Section>

        <Section title="课程自定义">
          {req.accountType === 'XUEXITONG' && (
            <div className="alert alert-warn" style={{marginBottom:10}}>
              GUI 启动支持普通/多课程/多任务点模式。CxNode 控制同一账号内同时进行的视频任务点数量；全局最大任务数只控制同时运行的账号数量。
            </div>
          )}
          <div className="form-row form-row-3">
            <FormGroup label="视频模式">
              <select className="form-select" value={req.coursesCustom.videoModel}
                onChange={e => setCC('videoModel', Number(e.target.value))}>
                <option value={0}>0 不刷</option>
                <option value={1}>1 普通（GUI 可用）</option>
                <option value={2}>2 多课程并发</option>
                <option value={3}>3 多任务点并发</option>
              </select>
            </FormGroup>
            <FormGroup label="自动答题方式">
              <select className="form-select" value={req.coursesCustom.autoExam}
                onChange={e => setCC('autoExam', Number(e.target.value))}>
                <option value={0}>0 关闭</option>
                <option value={1}>1 AI 答题</option>
                <option value={2}>2 外部题库</option>
                <option value={3}>3 免费AI（学习通）</option>
              </select>
            </FormGroup>
            <FormGroup label="自动提交试卷">
              <select className="form-select" value={req.coursesCustom.examAutoSubmit}
                onChange={e => setCC('examAutoSubmit', Number(e.target.value))}>
                <option value={0}>0 不提交</option>
                <option value={1}>1 提交</option>
                <option value={2}>2 智能提交（有空题时仅保存）</option>
              </select>
            </FormGroup>
            {req.coursesCustom.examAutoSubmit === 2 && (
              <FormGroup label="智能提交阈值 (%)">
                <input className="form-input" type="number" min={1} max={100} step={1}
                  value={!req.coursesCustom.submitThresholdPercent || req.coursesCustom.submitThresholdPercent <= 0 ? 100 : req.coursesCustom.submitThresholdPercent}
                  onChange={e => setCC('submitThresholdPercent', Math.min(100, Math.max(1, Number(e.target.value))))} />
                <div className="text-muted text-sm" style={{marginTop:3}}>
                  已答题数量达到总题数该百分比时提交（1-100）；未达到则只保存。100=全部答完才提交。
                </div>
              </FormGroup>
            )}
          </div>
          <div className="form-row form-row-2">
            <FormGroup label="包含课程（逗号分隔，空=全部）">
              <input className="form-input"
                value={(req.coursesCustom.includeCourses ?? []).join(',')}
                onChange={e => setCC('includeCourses', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
            </FormGroup>
            <FormGroup label="排除课程（逗号分隔）">
              <input className="form-input"
                value={(req.coursesCustom.excludeCourses ?? []).join(',')}
                onChange={e => setCC('excludeCourses', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
            </FormGroup>
          </div>
          <div className="form-row form-row-2">
            <FormGroup label="打乱顺序">
              <select className="form-select" value={req.coursesCustom.shuffleSw}
                onChange={e => setCC('shuffleSw', Number(e.target.value))}>
                <option value={0}>关闭</option><option value={1}>开启</option>
              </select>
            </FormGroup>
            <FormGroup label="WeLearn 学时时间范围">
              <input className="form-input" placeholder="如：10-30"
                value={req.coursesCustom.studyTime ?? ''}
                onChange={e => setCC('studyTime', e.target.value)} />
            </FormGroup>
            <FormGroup label="学习通同时任务点数（CxNode）">
              <input className="form-input" type="number" min={1} max={20}
                placeholder="默认 3，-1=全并发"
                value={req.coursesCustom.cxNode ?? 3}
                onChange={e => setCC('cxNode', Number(e.target.value))} />
              <div className="text-muted text-sm" style={{marginTop:3}}>
                控制同时进行的视频任务点数量，不支持按任务点过滤执行
              </div>
            </FormGroup>
            <FormGroup label="章节测验开关（CxChapterTestSw）">
              <select className="form-select" value={req.coursesCustom.cxChapterTestSw ?? 1}
                onChange={e => setCC('cxChapterTestSw', Number(e.target.value))}>
                <option value={0}>关闭</option><option value={1}>开启</option>
              </select>
            </FormGroup>
            <FormGroup label="作业开关（CxWorkSw）">
              <select className="form-select" value={req.coursesCustom.cxWorkSw ?? 1}
                onChange={e => setCC('cxWorkSw', Number(e.target.value))}>
                <option value={0}>关闭</option><option value={1}>开启</option>
              </select>
            </FormGroup>
            <FormGroup label="AI/题库失败后随机答选择题/判断题">
              <select className="form-select" value={req.coursesCustom.randomAnswerOnFail ?? 0}
                onChange={e => setCC('randomAnswerOnFail', Number(e.target.value))}>
                <option value={0}>关闭</option><option value={1}>开启</option>
              </select>
              <div className="text-muted text-sm" style={{marginTop:3}}>仅在已配置 AI 或题库、但未获取到有效答案时生效；只随机选择题和判断题。</div>
            </FormGroup>
            <FormGroup label="考试开关（CxExamSw）">
              <select className="form-select" value={req.coursesCustom.cxExamSw ?? 1}
                onChange={e => setCC('cxExamSw', Number(e.target.value))}>
                <option value={0}>关闭</option><option value={1}>开启</option>
              </select>
            </FormGroup>
          </div>
        </Section>

        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose}>取消</button>
          <button className="btn btn-primary" onClick={onSave} disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  )
}
