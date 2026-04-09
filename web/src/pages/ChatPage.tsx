import { useState } from 'react'
import { ContactList } from '../components/ContactList'
import { ChatWindow } from '../components/ChatWindow'
import { useWebSocket } from '../hooks/useWebSocket'
import { useStore } from '../store'
import { Zap, Copy, LogOut, MessageSquarePlus, X } from 'lucide-react'

export function ChatPage() {
  const { send } = useWebSocket()
  const username = useStore((s) => s.username)
  const userId = useStore((s) => s.userId)
  const clearAuth = useStore((s) => s.clearAuth)
  const [selectedContact, setSelectedContact] = useState<{ id: string; name: string } | null>(null)
  const [contactInput, setContactInput] = useState('')
  const [showNewChat, setShowNewChat] = useState(false)
  const [copied, setCopied] = useState(false)

  const dmMessages = useStore((s) => s.dmMessages)
  const contacts = Object.keys(dmMessages).map((id) => ({
    id,
    name: dmMessages[id].find((m) => m.from_id === id)?.from ?? id.slice(0, 8),
    lastMessage: dmMessages[id].at(-1)?.body,
    lastTime: dmMessages[id].at(-1)?.timestamp,
    unread: dmMessages[id].filter((m) => !m.mine && m.status !== 'read').length,
  }))

  function handleCopyId() {
    navigator.clipboard.writeText(userId ?? '')
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  function handleStartChat() {
    const trimmed = contactInput.trim()
    if (trimmed) {
      setSelectedContact({ id: trimmed, name: trimmed.slice(0, 8) })
      setContactInput('')
      setShowNewChat(false)
    }
  }

  return (
    <div style={{
      height: '100vh', display: 'flex', flexDirection: 'column',
      background: '#0d0d1a', fontFamily: "'JetBrains Mono', monospace",
      overflow: 'hidden',
    }}>

      {/* Top bar */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '10px 20px',
        background: 'rgba(255,255,255,0.02)',
        borderBottom: '1px solid rgba(99,102,241,0.15)',
        backdropFilter: 'blur(10px)',
        flexShrink: 0, zIndex: 10,
      }}>
        {/* Logo */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{
            width: 32, height: 32, borderRadius: 8,
            background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
            boxShadow: '0 0 16px rgba(99,102,241,0.5)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <Zap size={16} color="white" fill="white" />
          </div>
          <span style={{
            fontFamily: "'Orbitron', monospace", fontSize: 13, fontWeight: 900,
            letterSpacing: 3, color: '#fff',
            textShadow: '0 0 16px rgba(99,102,241,0.7)',
          }}>P2P CHAT</span>
        </div>

        {/* Right actions */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {/* Username */}
          <span style={{
            fontSize: 11, color: 'rgba(255,255,255,0.35)',
            background: 'rgba(99,102,241,0.08)',
            border: '1px solid rgba(99,102,241,0.15)',
            borderRadius: 8, padding: '4px 10px', letterSpacing: 0.5,
          }}>
            {username}
          </span>

          {/* Copy ID */}
          <button
            onClick={handleCopyId}
            title={`Your ID: ${userId}`}
            style={{
              display: 'flex', alignItems: 'center', gap: 5,
              padding: '5px 10px', borderRadius: 8, border: 'none', cursor: 'pointer',
              background: copied ? 'rgba(0,255,136,0.12)' : 'rgba(99,102,241,0.1)',
              color: copied ? '#00ff88' : 'rgba(255,255,255,0.5)',
              fontSize: 10, fontFamily: "'JetBrains Mono', monospace",
              letterSpacing: 0.5, transition: 'all 0.2s',
              boxShadow: copied ? '0 0 10px rgba(0,255,136,0.2)' : 'none',
            }}
          >
            <Copy size={11} />
            {copied ? 'COPIED!' : 'COPY ID'}
          </button>

          {/* Logout */}
          <button
            onClick={clearAuth}
            title="Logout"
            style={{
              width: 32, height: 32, borderRadius: 8, border: 'none', cursor: 'pointer',
              background: 'rgba(239,68,68,0.08)',
              color: 'rgba(239,68,68,0.5)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              transition: 'all 0.2s',
            }}
            onMouseEnter={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = 'rgba(239,68,68,0.18)'
              ;(e.currentTarget as HTMLButtonElement).style.color = '#ef4444'
            }}
            onMouseLeave={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = 'rgba(239,68,68,0.08)'
              ;(e.currentTarget as HTMLButtonElement).style.color = 'rgba(239,68,68,0.5)'
            }}
          >
            <LogOut size={14} />
          </button>
        </div>
      </div>

      {/* Main content */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>

        {/* Sidebar */}
        <div style={{
          width: 280, flexShrink: 0, display: 'flex', flexDirection: 'column',
          borderRight: '1px solid rgba(99,102,241,0.12)',
        }}>
          {/* New chat button */}
          <div style={{ padding: '12px 12px 0' }}>
            <button
              onClick={() => setShowNewChat((v) => !v)}
              style={{
                width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7,
                padding: '9px 0', borderRadius: 10, border: 'none', cursor: 'pointer',
                background: showNewChat
                  ? 'rgba(99,102,241,0.2)'
                  : 'linear-gradient(135deg, rgba(99,102,241,0.15), rgba(139,92,246,0.1))',
                color: '#818cf8', fontSize: 11, fontFamily: "'JetBrains Mono', monospace",
                fontWeight: 600, letterSpacing: 1,
                outline: '1px solid rgba(99,102,241,0.2)',
                transition: 'all 0.2s',
              }}
            >
              <MessageSquarePlus size={13} />
              NEW CHAT
            </button>

            {/* New chat input */}
            {showNewChat && (
              <div className="animate-slide-up" style={{ marginTop: 8, display: 'flex', gap: 6 }}>
                <input
                  type="text"
                  placeholder="Paste user ID..."
                  value={contactInput}
                  onChange={(e) => setContactInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleStartChat() }}
                  autoFocus
                  style={{
                    flex: 1, background: 'rgba(0,0,0,0.35)',
                    border: '1px solid rgba(99,102,241,0.2)',
                    borderRadius: 8, color: '#ededef', fontSize: 11,
                    fontFamily: "'JetBrains Mono', monospace",
                    padding: '8px 10px', outline: 'none',
                  }}
                  onFocus={(e) => { e.target.style.borderColor = 'rgba(99,102,241,0.5)' }}
                  onBlur={(e) => { e.target.style.borderColor = 'rgba(99,102,241,0.2)' }}
                />
                <button
                  onClick={handleStartChat}
                  style={{
                    padding: '8px 10px', borderRadius: 8, border: 'none', cursor: 'pointer',
                    background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                    color: 'white', fontSize: 11, fontFamily: "'JetBrains Mono', monospace",
                    letterSpacing: 0.5,
                    boxShadow: '0 0 10px rgba(99,102,241,0.3)',
                  }}
                >
                  GO
                </button>
                <button
                  onClick={() => { setShowNewChat(false); setContactInput('') }}
                  style={{
                    width: 32, height: 32, borderRadius: 8, border: 'none', cursor: 'pointer',
                    background: 'rgba(255,255,255,0.04)',
                    color: 'rgba(255,255,255,0.3)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                  }}
                >
                  <X size={12} />
                </button>
              </div>
            )}
          </div>

          {/* Contact list */}
          <div style={{ flex: 1, overflow: 'hidden', marginTop: 8 }}>
            <ContactList
              contacts={contacts}
              onSelect={(id, name) => setSelectedContact({ id, name })}
              selectedId={selectedContact?.id ?? null}
            />
          </div>
        </div>

        {/* Chat area */}
        <div style={{ flex: 1, overflow: 'hidden' }}>
          {selectedContact ? (
            <ChatWindow
              partnerId={selectedContact.id}
              partnerName={selectedContact.name}
              send={send}
            />
          ) : (
            <div style={{
              height: '100%', display: 'flex', flexDirection: 'column',
              alignItems: 'center', justifyContent: 'center', gap: 16,
            }}>
              {/* Decorative orb */}
              <div style={{
                width: 80, height: 80, borderRadius: '50%',
                background: 'radial-gradient(circle, rgba(99,102,241,0.2) 0%, transparent 70%)',
                border: '1px solid rgba(99,102,241,0.15)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                boxShadow: '0 0 40px rgba(99,102,241,0.1)',
              }}>
                <Zap size={32} color="#6366f1" style={{ filter: 'drop-shadow(0 0 8px rgba(99,102,241,0.8))' }} />
              </div>
              <div style={{ textAlign: 'center' }}>
                <p style={{ fontSize: 15, fontWeight: 600, color: 'rgba(255,255,255,0.6)', marginBottom: 6 }}>
                  No conversation selected
                </p>
                <p style={{ fontSize: 11, color: 'rgba(255,255,255,0.2)', letterSpacing: 0.5 }}>
                  Pick a chat from the sidebar or start a new one
                </p>
              </div>
              <button
                onClick={() => setShowNewChat(true)}
                style={{
                  display: 'flex', alignItems: 'center', gap: 7,
                  padding: '10px 20px', borderRadius: 10, border: 'none', cursor: 'pointer',
                  background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                  color: 'white', fontSize: 11, fontFamily: "'JetBrains Mono', monospace",
                  fontWeight: 600, letterSpacing: 1,
                  boxShadow: '0 0 20px rgba(99,102,241,0.3)',
                }}
              >
                <MessageSquarePlus size={13} />
                START A CHAT
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
