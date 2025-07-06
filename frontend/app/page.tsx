'use client'

import { useEffect, useState } from 'react'
import { useChatStore } from '@/lib/store'
import { Sidebar } from '@/components/sidebar'
import { ChatView } from '@/components/chat-view'
import { Button } from '@/components/ui/button'
import { Menu } from 'lucide-react'

export default function Home() {
  const { sessions, createSession } = useChatStore()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  useEffect(() => {
    // Create initial session if none exist
    if (sessions.length === 0) {
      createSession('Welcome Chat')
    }
  }, [])

  return (
    <div className="flex h-screen h-screen-safe bg-background">
      {/* Mobile sidebar backdrop */}
      {sidebarOpen && (
        <div 
          className="fixed inset-0 bg-black/50 z-40 lg:hidden" 
          onClick={() => setSidebarOpen(false)}
        />
      )}
      
      <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      
      <div className="flex-1 flex flex-col">
        {/* Mobile header */}
        <div className="lg:hidden border-b p-4 flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setSidebarOpen(true)}
          >
            <Menu className="h-5 w-5" />
          </Button>
          <h1 className="font-semibold">Paladin AI</h1>
        </div>
        
        <main className="flex-1">
          <ChatView />
        </main>
      </div>
    </div>
  )
}
