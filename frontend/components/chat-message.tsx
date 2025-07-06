import { Message } from '@/lib/store'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { cn } from '@/lib/utils'
import { User, Bot } from 'lucide-react'

interface ChatMessageProps {
  message: Message
}

export function ChatMessage({ message }: ChatMessageProps) {
  const isUser = message.role === 'user'

  return (
    <div
      className={cn(
        'flex w-full gap-4 px-4 py-6',
        isUser ? 'bg-background' : 'bg-muted/50'
      )}
    >
      <Avatar className="h-8 w-8 shrink-0">
        <AvatarFallback>
          {isUser ? <User className="h-4 w-4" /> : <Bot className="h-4 w-4" />}
        </AvatarFallback>
      </Avatar>
      <div className="flex-1 space-y-2">
        <div className="font-semibold text-sm">
          {isUser ? 'You' : 'Paladin AI'}
        </div>
        <div className="text-sm text-foreground/90 whitespace-pre-wrap">
          {message.content}
        </div>
      </div>
    </div>
  )
}