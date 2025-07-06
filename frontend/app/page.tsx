'use client'

import { useEffect } from 'react'
import { useChatStore } from '@/lib/store'
import { Sidebar } from '@/components/sidebar'
import { ChatView } from '@/components/chat-view'

export default function Home() {
  const { sessions, createSession } = useChatStore()

  useEffect(() => {
    // Create initial session if none exist
    if (sessions.length === 0) {
      createSession('Welcome Chat')
    }
  }, [])

  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1">
        <ChatView />
      </main>
    </div>
  )
}
