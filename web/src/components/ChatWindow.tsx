import { useState, useRef, useEffect } from 'react'
import { useStore } from '../store'
import { MessageBubble } from './MessageBubble'
import { TypingIndicator } from './TypingIndicator'
import { fetchHistory } from '../api'
import { v4 as uuidv4 } from 'uuid'
import { Send, Phone, Video } from 'lucide-react'

interface Props {
  partnerId: string
  partnerName: string
  send: (data: unknown) => void
}

const AVATAR_COLORS: [string, string][] = [
  ['#f43f5e', '#ec4899'], ['#8b5cf6', '#6366f1'], ['#06b6d4', '#3b82f6'],
  ['#f59e0b', '#ef4444'], ['#10b981', '#059669'], ['#a78bfa', '#c084fc'],
]

export function ChatWindow({ partnerId, partnerName, send }: Props) {
  const userId = useStore((s) => s.userId)
  const username = useStore((s) => s.username)
  const token = useStore((s) => s.token)
  const messages = useStore((s) => s.dmMessages[partnerId]) ?? []
  const isTyping = useStore((s) => s.typing[partnerId])
  const presence = useStore((s) => s.presence)
  const addDMMessage = useStore((s) => s.addDMMessage)
  const updateDMMessageStatus = useStore((s) => s.updateDMMessageStatus)
  const setDMHistory = useStore((s) => s.setDMHistory)

  const [body, setBody] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const ackedIds = useRef<Set<string>>(new Set())
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const isOnline = !!presence[partnerId]
  const [from, to] = AVATAR_COLORS[partnerName.charCodeAt(0) % AVATAR_COLORS.length]

  // Load history from server when opening a chat
  useEffect(() => {
    if (!token) return
    fetchHistory(partnerId, token).then((history) => {
      if (history.length === 0) return
      const mapped = history.map((m) => ({
        id: m.id,
        from: m.from_id === userId ? (username ?? '') : partnerName,
        from_id: m.from_id,
        body: m.body,
        timestamp: m.timestamp,
        status: (m.status ?? 'delivered') as 'sent' | 'delivered' | 'read',
        mine: m.from_id === userId,
      }))
      setDMHistory(partnerId, mapped)
    }).catch(() => {/* silently ignore — local messages still shown */})
  }, [partnerId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    messages.forEach((m) => {
      if (!m.mine && m.status !== 'read' && !ackedIds.current.has(m.id)) {
        ackedIds.current.add(m.id)
        send({ type: 'ack', message_id: m.id, sender_id: partnerId })
        // Mark as read locally so unread count clears immediately for the reader
        updateDMMessageStatus(partnerId, m.id, 'read')
      }
    })
  }, [messages, send, partnerId, updateDMMessageStatus])

  function handleTyping() {
    send({ type: 'typing', to: partnerId })
    if (typingTimer.current) clearTimeout(typingTimer.current)
    typingTimer.current = setTimeout(() => {}, 3000)
  }

  function handleSend() {
    if (!body.trim()) return
    const id = uuidv4()
    addDMMessage(partnerId, {
      id, from: username!, from_id: userId!,
      body: body.trim(),
      timestamp: new Date().toISOString(),
      status: 'sent', mine: true,
    })
    send({ type: 'message', to: partnerId, body: body.trim(), id })
    setBody('')
    textareaRef.current?.focus()
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  // Group messages by date
  const grouped: { date: string; msgs: typeof messages }[] = []
  messages.forEach((m) => {
    const date = new Date(m.timestamp).toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })
    if (!grouped.length || grouped[grouped.length - 1].date !== date) {
      grouped.push({ date, msgs: [m] })
    } else {
      grouped[grouped.length - 1].msgs.push(m)
    }
  })

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#0d0d1a' }}>

      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 12,
        padding: '14px 20px',
        background: 'rgba(255,255,255,0.025)',
        borderBottom: '1px solid rgba(99,102,241,0.12)',
        backdropFilter: 'blur(10px)',
        flexShrink: 0,
      }}>
        {/* Avatar */}
        <div style={{ position: 'relative' }}>
          <div style={{
            width: 40, height: 40, borderRadius: '50%',
            background: `linear-gradient(135deg, ${from}, ${to})`,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 16, fontWeight: 700, color: 'white',
            boxShadow: `0 0 14px ${from}55`,
          }}>
            {partnerName[0]?.toUpperCase()}
          </div>
          <span style={{
            position: 'absolute', bottom: 1, right: 1,
            width: 10, height: 10, borderRadius: '50%',
            border: '2px solid #0d0d1a',
            background: isOnline ? '#00ff88' : '#374151',
            boxShadow: isOnline ? '0 0 8px #00ff88, 0 0 16px rgba(0,255,136,0.4)' : 'none',
          }} />
        </div>

        {/* Name + status */}
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: '#fff', marginBottom: 2 }}>
            {partnerName}
          </div>
          <div style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 5 }}>
            {isTyping ? (
              <span style={{ color: '#818cf8', animation: 'glow-pulse 1.5s ease infinite' }}>typing...</span>
            ) : isOnline ? (
              <>
                <span style={{
                  width: 6, height: 6, borderRadius: '50%', background: '#00ff88',
                  boxShadow: '0 0 6px #00ff88', display: 'inline-block',
                }} />
                <span style={{ color: '#00ff88' }}>Online</span>
              </>
            ) : (
              <span style={{ color: 'rgba(255,255,255,0.3)' }}>Offline</span>
            )}
          </div>
        </div>

        {/* Action buttons */}
        <div style={{ display: 'flex', gap: 6 }}>
          {[Phone, Video].map((Icon, i) => (
            <button key={i} title={i === 0 ? 'Voice call' : 'Video call'} style={{
              width: 36, height: 36, borderRadius: '50%', border: 'none',
              background: 'rgba(99,102,241,0.12)',
              color: 'rgba(255,255,255,0.5)', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              transition: 'all 0.2s',
            }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'rgba(99,102,241,0.25)'
                ;(e.currentTarget as HTMLButtonElement).style.color = 'white'
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'rgba(99,102,241,0.12)'
                ;(e.currentTarget as HTMLButtonElement).style.color = 'rgba(255,255,255,0.5)'
              }}
            >
              <Icon size={16} />
            </button>
          ))}
        </div>
      </div>

      {/* Messages area */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 16px 8px' }}>
        {grouped.length === 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: 12 }}>
            <div style={{
              width: 64, height: 64, borderRadius: '50%',
              background: `linear-gradient(135deg, ${from}, ${to})`,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 26, fontWeight: 700, color: 'white',
              boxShadow: `0 0 30px ${from}55`,
            }}>{partnerName[0]?.toUpperCase()}</div>
            <p style={{ fontSize: 13, color: 'rgba(255,255,255,0.4)', textAlign: 'center' }}>
              Start a conversation with <span style={{ color: '#818cf8' }}>{partnerName}</span>
            </p>
          </div>
        )}

        {grouped.map(({ date, msgs }) => (
          <div key={date}>
            {/* Date separator */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, margin: '12px 0' }}>
              <div style={{ flex: 1, height: 1, background: 'rgba(99,102,241,0.1)' }} />
              <span style={{
                fontSize: 10, color: 'rgba(255,255,255,0.25)',
                background: 'rgba(99,102,241,0.08)', border: '1px solid rgba(99,102,241,0.12)',
                borderRadius: 20, padding: '3px 10px', letterSpacing: 0.5,
              }}>{date}</span>
              <div style={{ flex: 1, height: 1, background: 'rgba(99,102,241,0.1)' }} />
            </div>
            {msgs.map((m) => (
              <MessageBubble
                key={m.id}
                body={m.body}
                from={m.from}
                timestamp={m.timestamp}
                mine={m.from_id === userId}
                status={m.status}
              />
            ))}
          </div>
        ))}

        {isTyping && <TypingIndicator name={partnerName} />}
        <div ref={bottomRef} />
      </div>

      {/* Input area */}
      <div style={{
        padding: '12px 16px',
        borderTop: '1px solid rgba(99,102,241,0.1)',
        background: 'rgba(255,255,255,0.02)',
        flexShrink: 0,
      }}>
        <div style={{
          display: 'flex', gap: 10, alignItems: 'flex-end',
          background: 'rgba(0,0,0,0.35)',
          border: '1px solid rgba(99,102,241,0.2)',
          borderRadius: 16, padding: '10px 10px 10px 16px',
          transition: 'border-color 0.2s, box-shadow 0.2s',
        }}
          onFocusCapture={(e) => {
            (e.currentTarget as HTMLDivElement).style.borderColor = 'rgba(99,102,241,0.5)'
            ;(e.currentTarget as HTMLDivElement).style.boxShadow = '0 0 15px rgba(99,102,241,0.1)'
          }}
          onBlurCapture={(e) => {
            (e.currentTarget as HTMLDivElement).style.borderColor = 'rgba(99,102,241,0.2)'
            ;(e.currentTarget as HTMLDivElement).style.boxShadow = 'none'
          }}
        >
          <textarea
            ref={textareaRef}
            rows={1}
            placeholder={`Message ${partnerName}...`}
            value={body}
            onChange={(e) => { setBody(e.target.value); handleTyping() }}
            onKeyDown={handleKeyDown}
            style={{
              flex: 1, background: 'transparent', border: 'none', outline: 'none',
              color: '#ededef', fontSize: 15, resize: 'none', maxHeight: 120,
              fontFamily: "'JetBrains Mono', monospace", lineHeight: 1.5,
              paddingTop: 6, minHeight: 28,
            }}
          />
          <button
            onClick={handleSend}
            disabled={!body.trim()}
            style={{
              width: 44, height: 44, borderRadius: 12, border: 'none',
              background: body.trim()
                ? 'linear-gradient(135deg, #6366f1, #8b5cf6)'
                : 'rgba(99,102,241,0.15)',
              color: body.trim() ? 'white' : 'rgba(255,255,255,0.25)',
              cursor: body.trim() ? 'pointer' : 'not-allowed',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              flexShrink: 0, transition: 'all 0.2s',
              boxShadow: body.trim() ? '0 0 16px rgba(99,102,241,0.4)' : 'none',
            }}
          >
            <Send size={15} />
          </button>
        </div>
        <p style={{ fontSize: 10, color: 'rgba(255,255,255,0.15)', marginTop: 5, textAlign: 'center', letterSpacing: 0.5 }}>
          Enter to send · Shift+Enter for new line
        </p>
      </div>
    </div>
  )
}
