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
  clearAuth: () => set({
    token: null, userId: null, username: null,
    dmMessages: {}, groups: [], groupMessages: {},
  }),

  dmMessages: {},
  addDMMessage: (chatPartnerId, msg) =>
    set((s) => ({
      dmMessages: {
        ...s.dmMessages,
        [chatPartnerId]: [...(s.dmMessages[chatPartnerId] ?? []), msg],
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
