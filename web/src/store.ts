import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface Message {
  id: string
  from: string
  from_id: string
  body: string
  timestamp: string
  status: 'sent' | 'delivered' | 'read'
  mine: boolean
}

export interface GroupMessage {
  id: string
  from: string
  from_id: string
  body: string
  timestamp: string
  group_id: string
}

export interface Group {
  id: string
  name: string
  created_by: string
}

interface ChatStore {
  // Auth
  token: string | null
  userId: string | null
  username: string | null
  setAuth: (token: string, userId: string, username: string) => void
  clearAuth: () => void

  // DMs: keyed by the other user's ID
  dmMessages: Record<string, Message[]>
  addDMMessage: (chatPartnerId: string, msg: Message) => void
  // Replace a conversation's messages with server history (deduplicates by id)
  setDMHistory: (chatPartnerId: string, msgs: Message[]) => void
  // Swap client-generated UUID with server UUID and mark delivered
  confirmDMMessage: (chatPartnerId: string, clientId: string, serverMessageId: string) => void
  updateDMMessageStatus: (chatPartnerId: string, messageId: string, status: Message['status']) => void

  // Groups
  groups: Group[]
  setGroups: (groups: Group[]) => void
  groupMessages: Record<string, GroupMessage[]>
  addGroupMessage: (groupId: string, msg: GroupMessage) => void

  // Presence
  presence: Record<string, boolean>
  setPresence: (userId: string, online: boolean) => void

  // Typing
  typing: Record<string, boolean>
  setTyping: (chatId: string, on: boolean) => void

  // Active conversation
  activeChatId: string | null
  activeChatType: 'dm' | 'group' | null
  setActiveChat: (id: string, type: 'dm' | 'group') => void
}

export const useStore = create<ChatStore>()(persist((set) => ({
  token: null,
  userId: null,
  username: null,

  setAuth: (token, userId, username) => set({ token, userId, username }),
  // Only clear auth credentials — messages stay so history survives logout/login
  clearAuth: () => set({ token: null, userId: null, username: null }),

  dmMessages: {},
  addDMMessage: (chatPartnerId, msg) =>
    set((s) => ({
      dmMessages: {
        ...s.dmMessages,
        [chatPartnerId]: [...(s.dmMessages[chatPartnerId] ?? []), msg],
      },
    })),
  setDMHistory: (chatPartnerId, msgs) =>
    set((s) => {
      // Merge: server history as base, keep any local messages not in history (e.g. optimistic sends)
      const serverIds = new Set(msgs.map((m) => m.id))
      const localOnly = (s.dmMessages[chatPartnerId] ?? []).filter((m) => !serverIds.has(m.id))
      const merged = [...msgs, ...localOnly].sort(
        (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
      )
      return { dmMessages: { ...s.dmMessages, [chatPartnerId]: merged } }
    }),
  confirmDMMessage: (chatPartnerId, clientId, serverMessageId) =>
    set((s) => ({
      dmMessages: {
        ...s.dmMessages,
        [chatPartnerId]: (s.dmMessages[chatPartnerId] ?? []).map((m) =>
          m.id === clientId ? { ...m, id: serverMessageId, status: 'delivered' } : m
        ),
      },
    })),
  updateDMMessageStatus: (chatPartnerId, messageId, status) =>
    set((s) => ({
      dmMessages: {
        ...s.dmMessages,
        [chatPartnerId]: (s.dmMessages[chatPartnerId] ?? []).map((m) =>
          m.id === messageId ? { ...m, status } : m
        ),
      },
    })),

  groups: [],
  setGroups: (groups) => set({ groups }),
  groupMessages: {},
  addGroupMessage: (groupId, msg) =>
    set((s) => ({
      groupMessages: {
        ...s.groupMessages,
        [groupId]: [...(s.groupMessages[groupId] ?? []), msg],
      },
    })),

  presence: {},
  setPresence: (userId, online) =>
    set((s) => ({ presence: { ...s.presence, [userId]: online } })),

  typing: {},
  setTyping: (chatId, on) =>
    set((s) => ({ typing: { ...s.typing, [chatId]: on } })),

  activeChatId: null,
  activeChatType: null,
  setActiveChat: (id, type) => set({ activeChatId: id, activeChatType: type }),
}), { name: 'p2p-chat-store' }))
