import { useChatStore, ChatSession } from '@/lib/store'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { ThemeToggle } from '@/components/theme-toggle'
import { Plus, MessageSquare, Trash2, X } from 'lucide-react'
import { cn } from '@/lib/utils'

interface SidebarProps {
  isOpen?: boolean
  onClose?: () => void
}

export function Sidebar({ isOpen = true, onClose }: SidebarProps) {
  const { sessions, currentSessionId, createSession, deleteSession, setCurrentSession } = useChatStore()

  const handleNewChat = () => {
    createSession()
    onClose?.()
  }
  
  const handleSelectSession = (sessionId: string) => {
    setCurrentSession(sessionId)
    onClose?.()
  }

  const sortedSessions = [...sessions].sort((a, b) => {
    const dateA = a.updatedAt instanceof Date ? a.updatedAt : new Date(a.updatedAt)
    const dateB = b.updatedAt instanceof Date ? b.updatedAt : new Date(b.updatedAt)
    return dateB.getTime() - dateA.getTime()
  })

  return (
    <div className={cn(
      "fixed inset-y-0 left-0 z-50 flex h-full w-64 flex-col border-r bg-background transition-transform lg:relative lg:translate-x-0",
      isOpen ? "translate-x-0" : "-translate-x-full"
    )}>
      <div className="flex items-center justify-between p-4 lg:justify-center">
        <Button onClick={handleNewChat} className="flex-1 gap-2 lg:w-full">
          <Plus className="h-4 w-4" />
          New Chat
        </Button>
        {onClose && (
          <Button
            variant="ghost"
            size="icon"
            className="ml-2 lg:hidden"
            onClick={onClose}
          >
            <X className="h-4 w-4" />
          </Button>
        )}
      </div>

      <ScrollArea className="flex-1">
        <div className="space-y-1 p-2">
          {sortedSessions.map((session) => (
            <div
              key={session.id}
              className={cn(
                "group flex items-center justify-between rounded-lg px-3 py-2 hover:bg-accent cursor-pointer",
                currentSessionId === session.id && "bg-accent"
              )}
              onClick={() => handleSelectSession(session.id)}
            >
              <div className="flex items-center gap-2 overflow-hidden">
                <MessageSquare className="h-4 w-4 shrink-0" />
                <span className="truncate text-sm">{session.title}</span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6 opacity-0 group-hover:opacity-100"
                onClick={(e) => {
                  e.stopPropagation()
                  deleteSession(session.id)
                }}
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            </div>
          ))}
        </div>
      </ScrollArea>

      {/* Theme toggle at bottom */}
      <div className="border-t p-4 hidden lg:block">
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">Theme</span>
          <ThemeToggle />
        </div>
      </div>
    </div>
  )
}