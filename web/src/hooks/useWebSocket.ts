import { useEffect, useRef, useCallback } from 'react'
import { useStore } from '../store'

function checkPresence(ws: WebSocket, userIds: string[]) {
  if (ws.readyState === WebSocket.OPEN && userIds.length > 0) {
    ws.send(JSON.stringify({ type: 'presence_check', user_ids: userIds }))
  }
}

const WS_URL = import.meta.env.VITE_WS_URL ?? ''

export function useWebSocket() {
  const token = useStore((s) => s.token)
  const dmMessages = useStore((s) => s.dmMessages)
  const addDMMessage = useStore((s) => s.addDMMessage)
  const confirmDMMessage = useStore((s) => s.confirmDMMessage)
  const updateDMMessageStatus = useStore((s) => s.updateDMMessageStatus)
  const addGroupMessage = useStore((s) => s.addGroupMessage)
  const setPresence = useStore((s) => s.setPresence)
  const setTyping = useStore((s) => s.setTyping)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const reconnectDelay = useRef(1000)

  // Keep store callbacks in a ref so the effect never needs to re-run for them
  const cbRef = useRef({ addDMMessage, confirmDMMessage, updateDMMessageStatus, addGroupMessage, setPresence, setTyping, dmMessages })
  useEffect(() => {
    cbRef.current = { addDMMessage, confirmDMMessage, updateDMMessageStatus, addGroupMessage, setPresence, setTyping, dmMessages }
  })

  useEffect(() => {
    if (!token) return

    let cancelled = false

    function connect() {
      if (cancelled) return

      const url = `${WS_URL}/ws?token=${token}`
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => {
        reconnectDelay.current = 1000
        // Check presence for all existing contacts immediately on connect
        const contactIds = Object.keys(cbRef.current.dmMessages)
        checkPresence(ws, contactIds)
        const hb = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }))
            // Re-check presence every heartbeat
            const ids = Object.keys(cbRef.current.dmMessages)
            checkPresence(ws, ids)
          }
        }, 20000)
        ws.addEventListener('close', () => clearInterval(hb), { once: true })
      }

      ws.onmessage = (event) => {
        let msg: Record<string, unknown>
        try { msg = JSON.parse(event.data) } catch { return }
        const { addDMMessage, confirmDMMessage, addGroupMessage, setPresence, setTyping } = cbRef.current

        switch (msg.type) {
          case 'message': {
            const fromId = msg.from_id as string
            addDMMessage(fromId, {
              id: msg.message_id as string,
              from: msg.from as string,
              from_id: fromId,
              body: msg.body as string,
              timestamp: msg.timestamp as string,
              status: 'delivered',
              mine: false,
            })
            // Check presence for this contact if they just appeared
            checkPresence(ws, [fromId])
            break
          }
          case 'sent': {
            // Server confirmed our message — swap client UUID → server UUID, mark delivered (✓✓ grey)
            const clientId = msg.id as string
            const serverMessageId = msg.message_id as string
            const { dmMessages } = cbRef.current
            for (const partnerId of Object.keys(dmMessages)) {
              const found = dmMessages[partnerId]?.find((m) => m.id === clientId)
              if (found) {
                confirmDMMessage(partnerId, clientId, serverMessageId)
                break
              }
            }
            break
          }
          case 'read': {
            // Server tells us our message was read — update to blue ✓✓
            const messageId = msg.message_id as string
            const { dmMessages, updateDMMessageStatus } = cbRef.current
            for (const partnerId of Object.keys(dmMessages)) {
              const found = dmMessages[partnerId]?.find((m) => m.id === messageId)
              if (found) {
                updateDMMessageStatus(partnerId, messageId, 'read')
                break
              }
            }
            break
          }
          case 'group_message':
            addGroupMessage(msg.group_id as string, {
              id: msg.message_id as string,
              from: msg.from as string,
              from_id: msg.from_id as string,
              body: msg.body as string,
              timestamp: msg.timestamp as string,
              group_id: msg.group_id as string,
            })
            break
          case 'presence':
            setPresence(msg.user_id as string, msg.status === 'online')
            break
          case 'typing': {
            const chatId = (msg.chat_id ?? msg.group_id) as string
            setTyping(chatId, true)
            setTimeout(() => setTyping(chatId, false), 5000)
            break
          }
          case 'pong':
            break
        }
      }

      ws.onclose = () => {
        if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
        reconnectTimer.current = setTimeout(() => {
          reconnectDelay.current = Math.min(reconnectDelay.current * 2, 30000)
          connect()
        }, reconnectDelay.current)
      }
    }

    connect()

    return () => {
      cancelled = true
      wsRef.current?.close()
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
    }
  }, [token]) // only re-run when token changes

  const send = useCallback((data: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  return { send }
}
