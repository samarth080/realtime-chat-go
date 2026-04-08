import { useState } from 'react'
import { ContactList } from '../components/ContactList'
import { ChatWindow } from '../components/ChatWindow'
import { useWebSocket } from '../hooks/useWebSocket'
import { useStore } from '../store'

export function ChatPage() {
  const { send } = useWebSocket()
  const username = useStore((s) => s.username)
  const userId = useStore((s) => s.userId)
  const clearAuth = useStore((s) => s.clearAuth)
  const [selectedContact, setSelectedContact] = useState<{ id: string; name: string } | null>(null)
  const [contactInput, setContactInput] = useState('')

  const dmMessages = useStore((s) => s.dmMessages)
  const contacts = Object.keys(dmMessages).map((id) => ({
    id,
    name: dmMessages[id].find((m) => m.from_id === id)?.from ?? id.slice(0, 8),
  }))

  return (
    <div className="h-screen flex flex-col bg-gray-950">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-900 border-b border-gray-800">
        <span className="text-white font-semibold">P2P Chat — {username}</span>
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigator.clipboard.writeText(userId ?? '')}
            className="text-gray-400 hover:text-white text-xs border border-gray-700 rounded px-2 py-1 transition"
            title={userId ?? ''}
          >
            Copy my ID
          </button>
          <button onClick={clearAuth} className="text-gray-400 hover:text-white text-sm transition">
            Logout
          </button>
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        <div className="w-64 flex-shrink-0">
          <ContactList
            contacts={contacts}
            onSelect={(id, name) => setSelectedContact({ id, name })}
            selectedId={selectedContact?.id ?? null}
          />
        </div>

        <div className="flex-1">
          {selectedContact ? (
            <ChatWindow partnerId={selectedContact.id} partnerName={selectedContact.name} send={send} />
          ) : (
            <div className="h-full flex flex-col items-center justify-center text-gray-500">
              <p className="mb-4">Select a contact or start a new chat</p>
              <div className="flex gap-2">
                <input
                  className="bg-gray-800 text-white rounded-lg px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-indigo-500"
                  placeholder="Paste user ID..."
                  value={contactInput}
                  onChange={(e) => setContactInput(e.target.value)}
                />
                <button
                  onClick={() => {
                    if (contactInput.trim()) {
                      setSelectedContact({ id: contactInput.trim(), name: contactInput.trim().slice(0, 8) })
                      setContactInput('')
                    }
                  }}
                  className="bg-indigo-600 text-white px-3 py-2 rounded-lg text-sm hover:bg-indigo-500 transition"
                >
                  Chat
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
