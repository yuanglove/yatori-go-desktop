import { useRef, useEffect, useState } from 'react'

interface Props {
  show: boolean
  children: React.ReactNode
  /** extra className on the outer wrapper */
  className?: string
}

/**
 * Lightweight CSS-driven collapse using grid-template-rows trick.
 * No external deps. Respects prefers-reduced-motion via CSS.
 */
export default function Collapse({ show, children, className }: Props) {
  const [mounted, setMounted] = useState(show)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (show) {
      setMounted(true)
    } else {
      // keep DOM alive through the closing transition (240ms)
      timerRef.current = setTimeout(() => setMounted(false), 260)
    }
    return () => { if (timerRef.current) clearTimeout(timerRef.current) }
  }, [show])

  if (!mounted) return null

  return (
    <div
      className={'collapse-wrap' + (show ? ' collapse-open' : ' collapse-close') + (className ? ' ' + className : '')}
      aria-hidden={!show}
    >
      <div className="collapse-inner">
        {children}
      </div>
    </div>
  )
}
