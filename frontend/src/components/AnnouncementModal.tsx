import { Bell, ExternalLink, CheckCircle } from 'lucide-react'
import type { AnnouncementData } from '../lib/announcement'
import { markAnnouncementRead } from '../lib/announcement'

interface Props {
  data: AnnouncementData
  onClose: () => void
  openURL: (url: string) => void
}

const LEVEL_CLASS: Record<string, string> = {
  info: 'alert-info',
  warn: 'alert-warn',
  error: 'alert-warn',
}

export default function AnnouncementModal({ data, onClose, openURL }: Props) {
  const handleRead = () => {
    markAnnouncementRead(data.id)
    onClose()
  }

  const handleDetail = () => {
    if (data.url) openURL(data.url)
  }

  const levelCls = LEVEL_CLASS[data.level] ?? 'alert-info'

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-label={data.title}>
      <div className="modal" style={{ maxWidth: 480 }}>
        <div className="modal-title" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Bell size={16} strokeWidth={2} style={{ flexShrink: 0 }} />
          {data.title}
        </div>

        <div className={`alert ${levelCls}`} style={{ margin: '0 0 16px', whiteSpace: 'pre-wrap', wordBreak: 'break-word', lineHeight: 1.7 }}>
          {data.content}
        </div>

        {data.updatedAt && (
          <div className="text-muted text-sm" style={{ marginBottom: 14 }}>
            {'更新于 '}{data.updatedAt}
          </div>
        )}

        <div className="modal-footer">
          {data.url && (
            <button className="btn btn-ghost btn-sm" onClick={handleDetail}>
              <ExternalLink size={13} strokeWidth={2} style={{ marginRight: 4, verticalAlign: 'middle' }} />
              {'查看详情'}
            </button>
          )}
          <button className="btn btn-primary" onClick={handleRead}>
            <CheckCircle size={13} strokeWidth={2} style={{ marginRight: 5, verticalAlign: 'middle' }} />
            {'我知道了'}
          </button>
        </div>
      </div>
    </div>
  )
}
