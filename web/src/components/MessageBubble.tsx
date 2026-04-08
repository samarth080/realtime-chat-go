interface Props {
  body: string
  from: string
  timestamp: string
  mine: boolean
  status?: 'sent' | 'delivered' | 'read'
}

export function MessageBubble({ body, from, timestamp, mine, status }: Props) {
  const tick = status === 'read' || status === 'delivered' ? '✓✓' : '✓'
  const tickColor = status === 'read' ? 'text-blue-400' : 'text-gray-400'

  return (
    <div className={`flex ${mine ? 'justify-end' : 'justify-start'} mb-2`}>
      <div className={`max-w-xs lg:max-w-md px-4 py-2 rounded-2xl ${mine ? 'bg-indigo-600 text-white rounded-br-sm' : 'bg-gray-800 text-gray-100 rounded-bl-sm'}`}>
        {!mine && <p className="text-xs text-indigo-400 font-medium mb-1">{from}</p>}
        <p className="text-sm">{body}</p>
        <div className="flex items-center justify-end gap-1 mt-1">
          <span className="text-xs text-gray-400">{new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
          {mine && <span className={`text-xs ${tickColor}`}>{tick}</span>}
        </div>
      </div>
    </div>
  )
}
