import { useState } from 'react'
import { useStore } from '../store'
import { Search, Users } from 'lucide-react'

interface Contact {
  id: string
  name: string
  lastMessage?: string
  lastTime?: string
  unread?: number
}

interface Props {
  contacts: Contact[]
  onSelect: (id: string, name: string) => void
  selectedId: string | null
}

function formatTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - d.getTime()) / 86400000)
  if (diffDays === 0) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (diffDays === 1) return 'Yesterday'
  if (diffDays < 7) return d.toLocaleDateString([], { weekday: 'short' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

const AVATAR_COLORS: [string, string][] = [
  ['#f43f5e', '#ec4899'], ['#8b5cf6', '#6366f1'], ['#06b6d4', '#3b82f6'],
  ['#f59e0b', '#ef4444'], ['#10b981', '#059669'], ['#a78bfa', '#c084fc'],
]

function Avatar({ name, size = 42 }: { name: string; size?: number }) {
  const [from, to] = AVATAR_COLORS[name.charCodeAt(0) % AVATAR_COLORS.length]
  return (
    <div style={{
      width: size, height: size, borderRadius: '50%', flexShrink: 0,
      background: `linear-gradient(135deg, ${from}, ${to})`,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontSize: size * 0.38, fontWeight: 700, color: 'white',
      boxShadow: `0 0 12px ${from}55`,
    }}>
      {name[0]?.toUpperCase() ?? '?'}
    </div>
  )
}

export function ContactList({ contacts, onSelect, selectedId }: Props) {
  const presence = useStore((s) => s.presence)
  const [search, setSearch] = useState('')

  const filtered = contacts.filter((c) =>
    c.name.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div style={{
      height: '100%', display: 'flex', flexDirection: 'column',
      background: 'rgba(255,255,255,0.015)',
      borderRight: '1px solid rgba(99,102,241,0.12)',
    }}>
      {/* Header */}
      <div style={{ padding: '18px 16px 12px', borderBottom: '1px solid rgba(99,102,241,0.08)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
          <Users size={15} color="#6366f1" />
          <span style={{
            fontFamily: "'Orbitron', monospace", fontSize: 10, fontWeight: 700,
            color: '#6366f1', letterSpacing: 3,
            textShadow: '0 0 10px rgba(99,102,241,0.5)',
          }}>CHATS</span>
        </div>
        {/* Search bar */}
        <div style={{ position: 'relative' }}>
          <Search size={13} style={{
            position: 'absolute', left: 11, top: '50%', transform: 'translateY(-50%)',
            color: 'rgba(255,255,255,0.2)', pointerEvents: 'none',
          }} />
          <input
            type="text"
            placeholder="Search conversations..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{
              width: '100%', padding: '9px 12px 9px 32px',
              background: 'rgba(0,0,0,0.35)',
              border: '1px solid rgba(99,102,241,0.15)',
              borderRadius: 10, color: '#ededef', fontSize: 11,
              fontFamily: "'JetBrains Mono', monospace",
              outline: 'none', transition: 'border-color 0.2s, box-shadow 0.2s',
            }}
            onFocus={(e) => {
              e.target.style.borderColor = 'rgba(99,102,241,0.5)'
              e.target.style.boxShadow = '0 0 12px rgba(99,102,241,0.1)'
            }}
            onBlur={(e) => {
              e.target.style.borderColor = 'rgba(99,102,241,0.15)'
              e.target.style.boxShadow = 'none'
            }}
          />
        </div>
      </div>

      {/* List */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {filtered.length === 0 && (
          <div style={{ padding: '40px 16px', textAlign: 'center', color: 'rgba(255,255,255,0.2)', fontSize: 11 }}>
            {search ? 'No results found' : 'No conversations yet'}
          </div>
        )}
        {filtered.map((c) => {
          const isOnline = !!presence[c.id]
          const isSelected = selectedId === c.id
          return (
            <button
              key={c.id}
              onClick={() => onSelect(c.id, c.name)}
              style={{
                width: '100%', display: 'flex', alignItems: 'center', gap: 11,
                padding: '11px 16px', border: 'none', cursor: 'pointer',
                background: isSelected
                  ? 'linear-gradient(90deg, rgba(99,102,241,0.18), rgba(99,102,241,0.04))'
                  : 'transparent',
                borderLeft: `2px solid ${isSelected ? '#6366f1' : 'transparent'}`,
                transition: 'all 0.15s ease', textAlign: 'left',
              }}
              onMouseEnter={(e) => { if (!isSelected) (e.currentTarget as HTMLButtonElement).style.background = 'rgba(99,102,241,0.06)' }}
              onMouseLeave={(e) => { if (!isSelected) (e.currentTarget as HTMLButtonElement).style.background = 'transparent' }}
            >
              {/* Avatar + presence dot */}
              <div style={{ position: 'relative', flexShrink: 0 }}>
                <Avatar name={c.name} size={42} />
                <span style={{
                  position: 'absolute', bottom: 1, right: 1,
                  width: 11, height: 11, borderRadius: '50%',
                  border: '2px solid #0d0d1a',
                  background: isOnline ? '#00ff88' : '#374151',
                  boxShadow: isOnline ? '0 0 8px #00ff88, 0 0 16px rgba(0,255,136,0.4)' : 'none',
                  transition: 'all 0.3s',
                }} />
              </div>

              {/* Text */}
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 3 }}>
                  <span style={{
                    fontSize: 13, fontWeight: 600,
                    color: isSelected ? '#fff' : 'rgba(255,255,255,0.85)',
                    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '65%',
                  }}>{c.name}</span>
                  <span style={{ fontSize: 10, color: 'rgba(255,255,255,0.25)', flexShrink: 0 }}>
                    {formatTime(c.lastTime)}
                  </span>
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{
                    fontSize: 11, color: 'rgba(255,255,255,0.3)',
                    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    maxWidth: (c.unread ?? 0) > 0 ? '75%' : '100%',
                  }}>
                    {c.lastMessage ?? (isOnline ? '● Online' : 'Offline')}
                  </span>
                  {(c.unread ?? 0) > 0 && (
                    <span style={{
                      background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                      color: 'white', fontSize: 9, fontWeight: 700,
                      padding: '2px 6px', borderRadius: 10,
                      boxShadow: '0 0 8px rgba(99,102,241,0.5)', flexShrink: 0,
                    }}>{c.unread}</span>
                  )}
                </div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
