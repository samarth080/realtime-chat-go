import { Check, CheckCheck } from 'lucide-react'

interface Props {
  body: string
  from: string
  timestamp: string
  mine: boolean
  status?: 'sent' | 'delivered' | 'read'
}

export function MessageBubble({ body, from, timestamp, mine, status }: Props) {
  const time = new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  const Tick = () => {
    if (!mine) return null
    if (status === 'read') return (
      <CheckCheck size={12} color="#818cf8"
        style={{ filter: 'drop-shadow(0 0 4px rgba(99,102,241,0.9))', flexShrink: 0 }} />
    )
    if (status === 'delivered') return (
      <CheckCheck size={12} color="rgba(255,255,255,0.45)" style={{ flexShrink: 0 }} />
    )
    return <Check size={12} color="rgba(255,255,255,0.3)" style={{ flexShrink: 0 }} />
  }

  if (mine) {
    return (
      <div className="animate-slide-up" style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 3, paddingRight: 4 }}>
        <div style={{
          maxWidth: '68%',
          background: 'linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%)',
          borderRadius: '18px 18px 4px 18px',
          padding: '10px 13px 8px',
          boxShadow: '0 0 24px rgba(99,102,241,0.3), 0 4px 12px rgba(0,0,0,0.35)',
        }}>
          <p style={{
            fontSize: 13.5, color: 'rgba(255,255,255,0.95)',
            lineHeight: 1.55, wordBreak: 'break-word', marginBottom: 5,
          }}>{body}</p>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 4 }}>
            <span style={{ fontSize: 10, color: 'rgba(255,255,255,0.5)', letterSpacing: 0.2 }}>{time}</span>
            <Tick />
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="animate-slide-up" style={{ display: 'flex', justifyContent: 'flex-start', marginBottom: 3, paddingLeft: 4 }}>
      <div style={{
        maxWidth: '68%',
        background: 'rgba(255,255,255,0.055)',
        border: '1px solid rgba(99,102,241,0.14)',
        borderRadius: '18px 18px 18px 4px',
        padding: '10px 13px 8px',
        boxShadow: '0 2px 8px rgba(0,0,0,0.25)',
      }}>
        <p style={{ fontSize: 10, color: '#a78bfa', fontWeight: 600, marginBottom: 5, letterSpacing: 0.4 }}>
          {from}
        </p>
        <p style={{
          fontSize: 13.5, color: 'rgba(255,255,255,0.88)',
          lineHeight: 1.55, wordBreak: 'break-word', marginBottom: 5,
        }}>{body}</p>
        <span style={{ fontSize: 10, color: 'rgba(255,255,255,0.28)', letterSpacing: 0.2 }}>{time}</span>
      </div>
    </div>
  )
}
