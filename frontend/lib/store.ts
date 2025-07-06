import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface Message {
  id: string
  role: 'user' | 'assistant' | 'system' | 'command'
  content: string
  timestamp: Date
  metadata?: {
    type?: string
    [key: string]: any
  }
}

export interface ChatSession {
  id: string
  title: string
  messages: Message[]
  createdAt: Date
  updatedAt: Date
}

interface ChatStore {
  sessions: ChatSession[]
  currentSessionId: string | null
  
  // Actions
  createSession: (title?: string) => string
  deleteSession: (id: string) => void
  setCurrentSession: (id: string) => void
  addMessage: (sessionId: string, message: Omit<Message, 'id' | 'timestamp'>) => void
  updateSessionTitle: (sessionId: string, title: string) => void
  getCurrentSession: () => ChatSession | undefined
  getSession: (id: string) => ChatSession | undefined
  clearSession: (id: string) => void
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set, get) => ({
      sessions: [],
      currentSessionId: null,

      createSession: (title = 'New Chat') => {
        const sessionId = `chat_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
        const newSession: ChatSession = {
          id: sessionId,
          title,
          messages: [],
          createdAt: new Date(),
          updatedAt: new Date(),
        }
        
        set((state) => ({
          sessions: [...state.sessions, newSession],
          currentSessionId: sessionId,
        }))
        
        return sessionId
      },

      deleteSession: (id) => {
        set((state) => ({
          sessions: state.sessions.filter((s) => s.id !== id),
          currentSessionId: state.currentSessionId === id ? null : state.currentSessionId,
        }))
      },

      setCurrentSession: (id) => {
        set({ currentSessionId: id })
      },

      addMessage: (sessionId, message) => {
        const newMessage: Message = {
          ...message,
          id: `msg_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
          timestamp: new Date(),
        }

        set((state) => {
          const updatedSessions = state.sessions.map((session) => {
            if (session.id === sessionId) {
              const updatedMessages = [...session.messages, newMessage]
              let updatedTitle = session.title
              
              // Auto-update title from first user message if it's still the default
              if ((session.title === 'New Chat' || session.title === 'Welcome Chat') && 
                  message.role === 'user' && 
                  session.messages.length === 0) {
                // Extract first 50 characters for title, remove newlines
                updatedTitle = message.content
                  .replace(/\n/g, ' ')
                  .trim()
                  .substring(0, 50)
                if (updatedTitle.length === 50) {
                  updatedTitle += '...'
                }
                // Fallback if content is empty
                if (!updatedTitle) {
                  updatedTitle = 'New Chat'
                }
              }
              
              return {
                ...session,
                messages: updatedMessages,
                title: updatedTitle,
                updatedAt: new Date(),
              }
            }
            return session
          })
          
          return { sessions: updatedSessions }
        })
      },

      updateSessionTitle: (sessionId, title) => {
        set((state) => ({
          sessions: state.sessions.map((session) =>
            session.id === sessionId
              ? { ...session, title, updatedAt: new Date() }
              : session
          ),
        }))
      },

      getCurrentSession: () => {
        const state = get()
        return state.sessions.find((s) => s.id === state.currentSessionId)
      },

      getSession: (id) => {
        return get().sessions.find((s) => s.id === id)
      },

      clearSession: (id) => {
        set((state) => ({
          sessions: state.sessions.map((session) =>
            session.id === id
              ? { ...session, messages: [], updatedAt: new Date() }
              : session
          ),
        }))
      },
    }),
    {
      name: 'paladin-chat-storage',
      storage: {
        getItem: (name) => {
          const str = localStorage.getItem(name)
          if (!str) return null
          const data = JSON.parse(str)
          
          // Convert date strings back to Date objects
          if (data.state?.sessions) {
            data.state.sessions = data.state.sessions.map((session: any) => ({
              ...session,
              createdAt: new Date(session.createdAt),
              updatedAt: new Date(session.updatedAt),
              messages: session.messages.map((msg: any) => ({
                ...msg,
                timestamp: new Date(msg.timestamp),
              })),
            }))
          }
          
          return data
        },
        setItem: (name, value) => {
          localStorage.setItem(name, JSON.stringify(value))
        },
        removeItem: (name) => {
          localStorage.removeItem(name)
        },
      },
    }
  )
)