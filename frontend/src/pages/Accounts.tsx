import { useState } from 'react'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { api } from '../lib/api'
import type { AccountVO, AccountReq, CoursesCustom } from '../lib/api'
import { useAsync, Confirm, Section, FormGroup, Spinner } from '../components/shared'
import TagInput from '../components/TagInput'
import Collapse from '../components/Collapse'
import AnimatedSelect from '../components/AnimatedSelect'

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
  if (g === 'full')        return <span className="badge badge-full">{'完整支持'}</span>
  if (g === 'config-only') return <span className="badge badge-config">{'仅配置'}</span>
  return <span className="badge badge-none">{'暂不支持'}</span>
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
    setEditing({
      uid: a.uid, accountType: a.accountType, url: a.url,
      remarkName: a.remarkName ?? '', account: a.account,
      password: '', isProxy: a.isProxy, informEmails: a.informEmails,
      coursesCustom: a.coursesCustom,
    })
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
      <div className="flex-between" style={{ marginBottom: 18 }}>
        <div className="page-title" style={{ marginBottom: 0 }}>{'账号管理'}</div>
        <button className="btn btn-primary" onClick={openAdd}>
          <Plus size={14} strokeWidth={2.5} style={{ marginRight: 5, verticalAlign: 'middle' }} />
          {'添加账号'}
        </button>
      </div>

      {loading ? <Spinner /> : (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <table className="table">
            <thead>
              <tr>
                <th>{'平台'}</th><th>{'账号'}</th><th>{'备注'}</th><th>{'代理'}</th><th>{'GUI 支持'}</th><th>{'操作'}</th>
              </tr>
            </thead>
            <tbody>
              {(accounts ?? []).map(a => (
                <tr key={a.uid}>
                  <td><span className="badge badge-config">{a.accountType}</span></td>
                  <td style={{ fontWeight: 500 }}>{a.account}</td>
                  <td className="text-muted">{a.remarkName || '—'}</td>
                  <td className="text-muted">{a.isProxy ? '是' : '否'}</td>
                  <td>{guiBadge(a.guiSupport)}</td>
                  <td>
                    <div className="flex-row">
                      <button className="btn btn-ghost btn-sm" onClick={() => openEdit(a)}>
                        <Pencil size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
                        {'编辑'}
                      </button>
                      <button className="btn btn-danger btn-sm" onClick={() => setDelUid(a.uid)}>
                        <Trash2 size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
                        {'删除'}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {(accounts ?? []).length === 0 && (
                <tr><td colSpan={6} style={{ textAlign: 'center', padding: '32px 24px', color: 'var(--text3)' }}>
                  {'暂无账号，点击"添加账号"开始'}
                </td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {editing && (
        <AccountModal req={editing} onChange={setEditing}
          onSave={handleSave} onClose={() => setEditing(null)} saving={saving} error={err} />
      )}
      {delUid && (
        <Confirm msg="确认删除该账号？运行中的任务将被停止。"
          onOk={handleDelete} onCancel={() => setDelUid('')} />
      )}
    </div>
  )
}

function AccountModal({ req, onChange, onSave, onClose, saving, error }: {
  req: AccountReq; onChange(r: AccountReq): void
  onSave(): void; onClose(): void; saving: boolean; error: string
}) {
  const set = (k: keyof AccountReq, v: unknown) => onChange({ ...req, [k]: v })
  const setCC = (k: keyof CoursesCustom, v: unknown) =>
    onChange({ ...req, coursesCustom: { ...req.coursesCustom, [k]: v } })

  const urlLabel = req.accountType === 'YINGHUA' ? '平台 URL *（英华必填）' : '平台 URL'
  const urlPlaceholder = req.accountType === 'YINGHUA' ? '必填，如 https://xxx.yinghuaxuetang.com' : '部分平台必填'
  const pwdLabel = req.uid ? '密码（留空保留原密码）' : '密码 *'

  return (
    <div className="modal-overlay">
      <div className="modal">
        <div className="modal-title">{req.uid ? '编辑账号' : '添加账号'}</div>
        {error && <div className="alert alert-warn" style={{ marginBottom: 14 }}>{error}</div>}

        <Section title="基本信息">
          <div className="form-row form-row-2">
            <FormGroup label="平台类型 *">
              <AnimatedSelect
                value={req.accountType}
                options={PLATFORMS.map(p => ({ value: p.code, label: p.name + ' (' + p.code + ')' }))}
                onChange={v => set('accountType', String(v))}
              />
            </FormGroup>
            <FormGroup label={urlLabel}>
              <input className="form-input" value={req.url}
                placeholder={urlPlaceholder}
                onChange={e => set('url', e.target.value)} />
            </FormGroup>
            <FormGroup label="账号 *">
              <input className="form-input" value={req.account}
                onChange={e => set('account', e.target.value)} />
            </FormGroup>
            <FormGroup label={pwdLabel}>
              <input className="form-input" type="password" value={req.password}
                placeholder={req.uid ? '已保存密码，留空不修改' : ''}
                onChange={e => set('password', e.target.value)} />
            </FormGroup>
            <FormGroup label="备注名">
              <input className="form-input" value={req.remarkName}
                onChange={e => set('remarkName', e.target.value)} />
            </FormGroup>
            <FormGroup label="使用代理">
              <AnimatedSelect
                value={req.isProxy}
                options={[{ value: 0, label: '否' }, { value: 1, label: '是' }]}
                onChange={v => set('isProxy', Number(v))}
              />
            </FormGroup>
          </div>
          <FormGroup label="通知邮箱（多个用英文逗号分隔）">
            <input className="form-input"
              value={(req.informEmails ?? []).join(',')}
              onChange={e => set('informEmails', e.target.value.split(',').map(s => s.trim()).filter(Boolean))} />
          </FormGroup>
        </Section>

        <Section title="课程自定义">
          <Collapse show={req.accountType === 'XUEXITONG'}>
            <div className="alert alert-info" style={{ marginBottom: 12 }}>
              {'CxNode 控制同一账号内同时进行的视频任务点数量；全局最大任务数只控制同时运行的账号数量。'}
            </div>
          </Collapse>
          <div className="form-row form-row-3">
            <FormGroup label="视频模式">
              <AnimatedSelect
                value={req.coursesCustom.videoModel}
                options={[
                  { value: 0, label: '0 不刷' },
                  { value: 1, label: '1 普通（GUI 可用）' },
                  { value: 2, label: '2 多课程并发' },
                  { value: 3, label: '3 多任务点并发' },
                ]}
                onChange={v => setCC('videoModel', Number(v))}
              />
            </FormGroup>
            <FormGroup label="">
              <div className="form-label form-label-row">
                <span>{'自动答题方式'}</span>
                {req.accountType === 'XUEXITONG' && req.coursesCustom.autoExam === 3 && (
                  <span className="field-help field-help-danger" tabIndex={0} aria-label="免费AI说明">
                    {'!'}
                    <span className="field-help-popover">
                      {'免费AI为原项目内置能力，稳定性和可用性取决于原接口状态；如果获取不到答案，可开启"AI/题库失败后随机答选择题/判断题"兜底。'}
                    </span>
                  </span>
                )}
              </div>
              <AnimatedSelect
                value={req.coursesCustom.autoExam}
                options={[
                  { value: 0, label: '0 关闭' },
                  { value: 1, label: '1 AI 答题' },
                  { value: 2, label: '2 外部题库' },
                  ...(req.accountType === 'XUEXITONG' ? [{ value: 3, label: '3 免费AI / 学习通内置AI' }] : []),
                ]}
                onChange={v => setCC('autoExam', Number(v))}
              />
              <Collapse show={req.accountType !== 'XUEXITONG' && req.coursesCustom.autoExam === 3}>
                <div className="alert alert-warn" style={{ marginTop: 4, fontSize: 12 }}>
                  {'免费AI仅对学习通（XUEXITONG）平台有效，当前平台不支持此选项，请切换其他答题方式。'}
                </div>
              </Collapse>
            </FormGroup>
            <FormGroup label="自动提交试卷">
              <AnimatedSelect
                value={req.coursesCustom.examAutoSubmit}
                options={[
                  { value: 0, label: '0 不提交' },
                  { value: 1, label: '1 提交' },
                  { value: 2, label: '2 智能提交' },
                ]}
                onChange={v => setCC('examAutoSubmit', Number(v))}
              />
            </FormGroup>
            <Collapse show={req.coursesCustom.examAutoSubmit === 2}>
              <FormGroup label="智能提交阈值 (%)">
                <input className="form-input" type="number" min={1} max={100}
                  value={!req.coursesCustom.submitThresholdPercent || req.coursesCustom.submitThresholdPercent <= 0
                    ? 100 : req.coursesCustom.submitThresholdPercent}
                  onChange={e => setCC('submitThresholdPercent', Math.min(100, Math.max(1, Number(e.target.value))))} />
                <div className="text-muted text-sm" style={{ marginTop: 3 }}>
                  {'已答题数量达到总题数该百分比时提交；未达到则只保存。'}
                </div>
              </FormGroup>
            </Collapse>
            <FormGroup label="打乱顺序">
              <AnimatedSelect
                value={req.coursesCustom.shuffleSw}
                options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
                onChange={v => setCC('shuffleSw', Number(v))}
              />
            </FormGroup>
            <FormGroup label="WeLearn 学时时间范围">
              <input className="form-input" placeholder="如：10-30"
                value={req.coursesCustom.studyTime ?? ''}
                onChange={e => setCC('studyTime', e.target.value)} />
            </FormGroup>
            <FormGroup label="CxNode（同时任务点数）">
              <input className="form-input" type="number" min={1} max={20}
                placeholder="默认 3"
                value={req.coursesCustom.cxNode ?? 3}
                onChange={e => setCC('cxNode', Number(e.target.value))} />
            </FormGroup>
            <FormGroup label="章节测验（CxChapterTestSw）">
              <AnimatedSelect
                value={req.coursesCustom.cxChapterTestSw ?? 1}
                options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
                onChange={v => setCC('cxChapterTestSw', Number(v))}
              />
            </FormGroup>
            <FormGroup label="作业（CxWorkSw）">
              <AnimatedSelect
                value={req.coursesCustom.cxWorkSw ?? 1}
                options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
                onChange={v => setCC('cxWorkSw', Number(v))}
              />
            </FormGroup>
            <FormGroup label="考试（CxExamSw）">
              <AnimatedSelect
                value={req.coursesCustom.cxExamSw ?? 1}
                options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
                onChange={v => setCC('cxExamSw', Number(v))}
              />
            </FormGroup>
            <FormGroup label="AI/题库失败后随机答题">
              <AnimatedSelect
                value={req.coursesCustom.randomAnswerOnFail ?? 0}
                options={[{ value: 0, label: '关闭' }, { value: 1, label: '开启' }]}
                onChange={v => setCC('randomAnswerOnFail', Number(v))}
              />
              <div className="text-muted text-sm" style={{ marginTop: 3 }}>
                {'仅在 AI/题库无有效答案时生效；只随机选择题和判断题。'}
              </div>
            </FormGroup>
          </div>
          <div className="form-row form-row-2" style={{ marginTop: 4 }}>
            <FormGroup label="包含课程（空=全部）">
              <TagInput
                value={req.coursesCustom.includeCourses ?? []}
                onChange={tags => setCC('includeCourses', tags)}
                placeholder={'输入课程名，Enter 或逗号确认'}
                variant="include"
              />
            </FormGroup>
            <FormGroup label="排除课程">
              <TagInput
                value={req.coursesCustom.excludeCourses ?? []}
                onChange={tags => setCC('excludeCourses', tags)}
                placeholder={'输入课程名，Enter 或逗号确认'}
                variant="exclude"
              />
            </FormGroup>
          </div>
        </Section>

        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose}>{'取消'}</button>
          <button className="btn btn-primary" onClick={onSave} disabled={saving}>
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>
    </div>
  )
}
