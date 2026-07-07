import { useState } from 'react'
import { ExternalLink, Plus, Pencil, Trash2 } from 'lucide-react'
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

const ICVE_COOKIE_DOC_URL = 'https://yatori-dev.github.io/yatori-docs/yatori-go-console/docs.html'

interface VideoModeOption { value: number; label: string }

const VIDEO_MODE_OPTIONS: Record<string, VideoModeOption[]> = {
  XUEXITONG: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
    { value: 2, label: '2 多课程并发' },
    { value: 3, label: '3 多任务点并发' },
  ],
  YINGHUA: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
    { value: 2, label: '2 快速模式' },
    { value: 3, label: '3 去红模式' },
  ],
  CANGHUI: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
    { value: 2, label: '2 快速模式' },
    { value: 3, label: '3 去红模式' },
  ],
  WELEARN: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 刷学习时长' },
    { value: 2, label: '2 刷完成度' },
  ],
  HQKJ: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
    { value: 2, label: '2 快速模式' },
  ],
  ENAEA: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
    { value: 2, label: '2 暴力模式' },
  ],
  CQIE: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
    { value: 2, label: '2 暴力模式（秒刷）' },
  ],
  QSXT: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 刷学时' },
  ],
  ICVE: [
    { value: 0, label: '0 不刷视频' },
    { value: 1, label: '1 默认秒刷' },
  ],
  KETANGX: [
    { value: 0, label: '0 不刷' },
    { value: 1, label: '1 普通模式' },
  ],
}

const DEFAULT_VIDEO_MODE_OPTIONS: VideoModeOption[] = [
  { value: 0, label: '0 不刷' },
  { value: 1, label: '1 普通模式' },
]

function getVideoModeOptions(platform: string): VideoModeOption[] {
  return VIDEO_MODE_OPTIONS[platform] ?? DEFAULT_VIDEO_MODE_OPTIONS
}

const VIDEO_MODE_HINTS: Record<string, string> = {
  XUEXITONG: '1=普通，2=多课程并发，3=多任务点并发',
  YINGHUA:   '1=普通，2=快速，3=去红模式（自动处理红色答题标记）',
  CANGHUI:   '1=普通，2=快速，3=去红模式（自动处理红色答题标记）',
  WELEARN:   '1=刷学习时长（每60s推进），2=刷完成度（直接标完成）',
  HQKJ:      '1=普通（逐步推进进度），2=快速（并发秒刷）',
  ENAEA:     '1=普通，2=暴力模式（强制提交学时）',
  CQIE:      '1=普通，2=暴力模式（秒刷）',
  QSXT:      '1=刷学时（仅支持单一模式）',
  ICVE:      '0=不刷视频但仍处理章节测验/文档等任务点，1=默认秒刷',
  KETANGX:   '1=普通模式（仅支持单一模式）',
}

function getVideoModeHint(platform: string): string {
  return VIDEO_MODE_HINTS[platform] ?? ''
}

/** 切换平台时，若 videoModel 不在新平台支持列表中，返回修正后的默认值（优先1，否则第一个选项） */
function clampVideoModel(platform: string, current: number): number {
  const opts = getVideoModeOptions(platform)
  if (opts.some(o => o.value === current)) return current
  return opts.find(o => o.value === 1)?.value ?? opts[0].value
}

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
    const cc = a.coursesCustom
    const fixedModel = clampVideoModel(a.accountType, cc.videoModel)
    setEditing({
      uid: a.uid, accountType: a.accountType, url: a.url,
      remarkName: a.remarkName ?? '', account: a.account,
      password: '', isProxy: a.isProxy, informEmails: a.informEmails,
      coursesCustom: { ...cc, videoModel: fixedModel },
    })
  }

  const handleSave = async () => {
    if (!editing) return
    if (editing.accountType === 'YINGHUA' && !editing.url.trim()) {
      setErr('英华平台必须填写学校入口地址（平台 URL）'); return
    }
    if (editing.accountType === 'ICVE' && !editing.uid && editing.password.trim().length <= 30) {
      setErr('智慧职教只支持 Cookie 登录，请把浏览器复制的完整 Cookie 填到密码/Cookie 字段'); return
    }
    if (editing.accountType === 'ICVE' && editing.uid && editing.password.trim() && editing.password.trim().length <= 30) {
      setErr('智慧职教只支持 Cookie 登录，若要修改请填写完整 Cookie；不修改请留空'); return
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
  const [cookieBusy, setCookieBusy] = useState(false)
  const [cookieMsg, setCookieMsg] = useState('')
  const set = (k: keyof AccountReq, v: unknown) => onChange({ ...req, [k]: v })
  const setCC = (k: keyof CoursesCustom, v: unknown) =>
    onChange({ ...req, coursesCustom: { ...req.coursesCustom, [k]: v } })

  const urlLabel = req.accountType === 'YINGHUA' ? '平台 URL *（英华必填）' : '平台 URL'
  const urlPlaceholder = req.accountType === 'YINGHUA' ? '必填，如 https://xxx.yinghuaxuetang.com' : '部分平台必填'
  const pwdLabel = req.accountType === 'ICVE'
    ? (req.uid ? 'Cookie（留空保留原 Cookie）' : 'Cookie *')
    : (req.uid ? '密码（留空保留原密码）' : '密码 *')
  const pwdPlaceholder = req.accountType === 'ICVE'
    ? (req.uid ? '已保存 Cookie，留空不修改' : '粘贴智慧职教 index 请求里的完整 Cookie')
    : (req.uid ? '已保存密码，留空不修改' : '')
  const startICVECookieCapture = async () => {
    setCookieBusy(true)
    setCookieMsg('正在打开独立浏览器窗口…')
    const r = await api.startICVECookieCapture(req.url || 'https://www.icve.com.cn/')
    setCookieBusy(false)
    if (!r.ok) { setCookieMsg(r.error ?? '启动失败'); return }
    setCookieMsg('浏览器已打开。请在该窗口登录智慧职教，登录成功后进入课程/首页并刷新一次，再回到这里点击“已登录，读取 Cookie”。')
  }
  const readICVECookie = async () => {
    setCookieBusy(true)
    setCookieMsg('正在读取 Cookie…')
    const r = await api.readICVECookie()
    setCookieBusy(false)
    if (!r.ok) { setCookieMsg(r.error ?? '读取失败'); return }
    set('password', r.data)
    setCookieMsg('Cookie 已自动填入，确认账号信息后点击保存。若启动任务提示 Cookie 失效，请重新获取一次。')
  }

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
                onChange={v => {
                  const platform = String(v)
                  const fixedModel = clampVideoModel(platform, req.coursesCustom.videoModel)
                  const nextCC = { ...req.coursesCustom, videoModel: fixedModel }
                  onChange({
                    ...req,
                    accountType: platform,
                    coursesCustom: nextCC,
                  })
                }}
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
            <FormGroup label={req.accountType === 'ICVE' ? (
              <span className="form-label-inline">
                <span>{pwdLabel}</span>
                <button
                  type="button"
                  className="inline-link-btn"
                  title="打开智慧职教 Cookie 获取教程"
                  onClick={() => window.open(ICVE_COOKIE_DOC_URL, '_blank', 'noopener,noreferrer')}
                >
                  <ExternalLink size={12} strokeWidth={2.2} />
                  {'获取教程'}
                </button>
              </span>
            ) : pwdLabel}>
              <input className="form-input" type="password" value={req.password}
                placeholder={pwdPlaceholder}
                onChange={e => set('password', e.target.value)} />
              {req.accountType === 'ICVE' && (
                <>
                  <div className="text-muted text-sm" style={{ marginTop: 3 }}>
                    {'智慧职教只支持 Cookie 登录：可手动粘贴 Cookie，也可用独立浏览器窗口自动获取。'}
                  </div>
                  <div className="flex-row" style={{ marginTop: 8, flexWrap: 'wrap' }}>
                    <button type="button" className="btn btn-ghost btn-sm" onClick={startICVECookieCapture} disabled={cookieBusy}>
                      {'自动获取 Cookie'}
                    </button>
                    <button type="button" className="btn btn-ghost btn-sm" onClick={readICVECookie} disabled={cookieBusy}>
                      {'已登录，读取 Cookie'}
                    </button>
                  </div>
                  {cookieMsg && <div className="alert alert-info" style={{ marginTop: 8, fontSize: 12 }}>{cookieMsg}</div>}
                </>
              )}
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
                options={getVideoModeOptions(req.accountType)}
                onChange={v => setCC('videoModel', Number(v))}
              />
              {getVideoModeHint(req.accountType) && (
                <div className="text-muted text-sm" style={{ marginTop: 3 }}>
                  {getVideoModeHint(req.accountType)}
                </div>
              )}
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
                  { value: 0, label: '0 保存不提交' },
                  { value: 1, label: '1 直接提交' },
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
                {'仅在 AI/题库无有效答案时生效；随机选择题、多选题和判断题，填空/简答/论述不随机。'}
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
