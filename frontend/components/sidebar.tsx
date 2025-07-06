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

  // Helper function to get date label
  const getDateLabel = (date: Date) => {
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000)
    const sessionDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())

    if (sessionDate.getTime() === today.getTime()) {
      return 'Today'
    } else if (sessionDate.getTime() === yesterday.getTime()) {
      return 'Yesterday'
    } else {
      return sessionDate.toLocaleDateString('en-GB', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric'
      })
    }
  }

  // Helper function to get time string
  const getTimeString = (date: Date) => {
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    })
  }

  // Group sessions by date
  const groupedSessions = sessions.reduce((groups: Record<string, ChatSession[]>, session) => {
    const date = session.updatedAt instanceof Date ? session.updatedAt : new Date(session.updatedAt)
    const dateLabel = getDateLabel(date)
    
    if (!groups[dateLabel]) {
      groups[dateLabel] = []
    }
    groups[dateLabel].push(session)
    return groups
  }, {})

  // Sort sessions within each group and sort groups
  const sortedGroups = Object.entries(groupedSessions).map(([dateLabel, sessions]) => ({
    dateLabel,
    sessions: sessions.sort((a, b) => {
      const dateA = a.updatedAt instanceof Date ? a.updatedAt : new Date(a.updatedAt)
      const dateB = b.updatedAt instanceof Date ? b.updatedAt : new Date(b.updatedAt)
      return dateB.getTime() - dateA.getTime()
    })
  })).sort((a, b) => {
    // Sort groups: Today -> Yesterday -> DD-MM-YYYY (newest first)
    if (a.dateLabel === 'Today') return -1
    if (b.dateLabel === 'Today') return 1
    if (a.dateLabel === 'Yesterday') return -1
    if (b.dateLabel === 'Yesterday') return 1
    // For date strings, sort by the most recent session in each group
    const latestA = Math.max(...a.sessions.map(s => (s.updatedAt instanceof Date ? s.updatedAt : new Date(s.updatedAt)).getTime()))
    const latestB = Math.max(...b.sessions.map(s => (s.updatedAt instanceof Date ? s.updatedAt : new Date(s.updatedAt)).getTime()))
    return latestB - latestA
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
        <div className="p-2">
          {sortedGroups.map((group, groupIndex) => (
            <div key={group.dateLabel} className="mb-4">
              {/* Date header */}
              <div className="sticky top-0 bg-background/95 backdrop-blur-sm px-2 py-1 mb-2">
                <h3 className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  {group.dateLabel}
                </h3>
              </div>
              
              {/* Sessions in this date group */}
              <div className="space-y-1">
                {group.sessions.map((session) => {
                  const sessionDate = session.updatedAt instanceof Date ? session.updatedAt : new Date(session.updatedAt)
                  return (
                    <div
                      key={session.id}
                      className={cn(
                        "group flex items-start justify-between rounded-lg px-3 py-2 hover:bg-accent cursor-pointer relative",
                        currentSessionId === session.id && "bg-accent"
                      )}
                      onClick={() => handleSelectSession(session.id)}
                    >
                      <div className="flex items-center gap-2 overflow-hidden min-w-0 flex-1">
                        <MessageSquare className="h-4 w-4 shrink-0" />
                        <div className="min-w-0 flex-1">
                          <span className="truncate text-sm block">{session.title}</span>
                          <span className="text-xs text-muted-foreground">
                            {getTimeString(sessionDate)}
                          </span>
                        </div>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 opacity-0 group-hover:opacity-100 shrink-0"
                        onClick={(e) => {
                          e.stopPropagation()
                          deleteSession(session.id)
                        }}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  )
                })}
              </div>
              
              {/* Separator line (except for last group) */}
              {groupIndex < sortedGroups.length - 1 && (
                <div className="border-t my-4 mx-2" />
              )}
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