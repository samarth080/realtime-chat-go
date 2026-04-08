import { useState, useRef, useEffect } from 'react'
import { useStore } from '../store'
import { MessageBubble } from './MessageBubble'
import { TypingIndicator } from './TypingIndicator'
import { v4 as uuidv4 } from 'uuid'

interface Props {
  partnerId: string
  partnerName: string
  send: (data: unknown) => void
}

export function ChatWindow({ partnerId, partnerName, send }: Props) {
  const userId = useStore((s) => s.userId)
  const username = useStore((s) => s.username)
  const messages = useStore((s) => s.dmMessages[partnerId] ?? [])
  const isTyping = useStore((s) => s.typing[partnerId])
  const addDMMessage = useStore((s) => s.addDMMessage)
  const [body, setBody] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  function handleTyping() {
    send({ type: 'typing', to: partnerId })
    if (typingTimer.current) clearTimeout(typingTimer.current)
    typingTimer.current = setTimeout(() => {}, 3000)
  }

  function handleSend() {
    if (!body.trim()) return
    const id = uuidv4()
    addDMMessage(userId!, {
      id,
      from: username!,
      from_id: userId!,
      body,
      timestamp: new Date().toISOString(),
      status: 'sent',
      mine: true,
    })
    send({ type: 'message', to: partnerId, body, id })
    setBody('')
  }

  return (
    <div className="flex flex-col h-full bg-gray-950">
      <div className="px-4 py-3 border-b border-gray-800 bg-gray-900 font-medium text-white">
        {partnerName}
      </div>
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-1">
        {messages.map((m) => (
          <MessageBubble
            key={m.id}
            body={m.body}
            from={m.from}
            timestamp={m.timestamp}
            mine={m.from_id === userId}
            status={m.status}
          />
        ))}
        {isTyping && <TypingIndicator name={partnerName} />}
        <div ref={bottomRef} />
      </div>
      <div className="px-4 py-3 border-t border-gray-800 flex gap-2">
        <input
          className="flex-1 bg-gray-800 text-white rounded-xl px-4 py-2 outline-none focus:ring-2 focus:ring-indigo-500"
          placeholder="Message..."
          value={body}
          onChange={(e) => { setBody(e.target.value); handleTyping() }}
          onKeyDown={(e) => e.key === 'Enter' && handleSend()}
        />
        <button
          onClick={handleSend}
          className="bg-indigo-600 hover:bg-indigo-500 text-white px-4 py-2 rounded-xl transition"
        >
          Send
        </button>
      </div>
    </div>
  )
}
