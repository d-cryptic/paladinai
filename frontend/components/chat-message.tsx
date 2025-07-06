import { Message } from '@/lib/store'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { cn } from '@/lib/utils'
import { User, Bot, Brain } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

interface ChatMessageProps {
  message: Message
}

export function ChatMessage({ message }: ChatMessageProps) {
  const isUser = message.role === 'user'
  const isSystem = message.role === 'system'
  const isCommand = message.role === 'command'

  // Don't render command messages here - they're handled by CommandMessage
  if (isCommand) {
    return null
  }

  return (
    <div
      className={cn(
        'flex w-full gap-2 sm:gap-4 px-3 sm:px-4 py-3 sm:py-6 max-w-full overflow-hidden',
        isUser ? 'bg-background' : isSystem ? 'bg-blue-500/10' : 'bg-muted/50'
      )}
    >
      <Avatar className="h-6 w-6 sm:h-8 sm:w-8 shrink-0">
        <AvatarFallback>
          {isUser ? (
            <User className="h-4 w-4" />
          ) : isSystem ? (
            <Brain className="h-4 w-4" />
          ) : (
            <Bot className="h-4 w-4" />
          )}
        </AvatarFallback>
      </Avatar>
      <div className="flex-1 space-y-1 sm:space-y-2 min-w-0">
        <div className="font-semibold text-xs sm:text-sm">
          {isUser ? 'You' : isSystem ? 'Memory Context' : 'Paladin AI'}
        </div>
        <div className="text-xs sm:text-sm text-foreground/90 break-words">
          {isUser ? (
            <div className="whitespace-pre-wrap">{message.content}</div>
          ) : (
            <div className="prose prose-sm dark:prose-invert max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  pre: ({ children }) => (
                    <pre className="bg-muted p-2 sm:p-3 rounded-md overflow-x-auto my-2 text-xs sm:text-sm max-w-full">
                      {children}
                    </pre>
                  ),
                  code: ({ children, className }) => {
                    const isInline = !className
                    return isInline ? (
                      <code className="bg-muted px-1 py-0.5 rounded text-sm">{children}</code>
                    ) : (
                      <code className="text-sm">{children}</code>
                    )
                  },
                  h1: ({ children }) => <h1 className="text-lg sm:text-xl font-bold mt-3 sm:mt-4 mb-2">{children}</h1>,
                  h2: ({ children }) => <h2 className="text-base sm:text-lg font-semibold mt-2 sm:mt-3 mb-1 sm:mb-2">{children}</h2>,
                  h3: ({ children }) => <h3 className="text-sm sm:text-base font-semibold mt-2 mb-1">{children}</h3>,
                  p: ({ children }) => <p className="my-2">{children}</p>,
                  ul: ({ children }) => <ul className="list-disc list-inside ml-4 my-2">{children}</ul>,
                  ol: ({ children }) => <ol className="list-decimal list-inside ml-4 my-2">{children}</ol>,
                  li: ({ children }) => <li className="my-1">{children}</li>,
                  hr: () => <hr className="my-4 border-t border-muted-foreground/20" />,
                  strong: ({ children }) => <strong className="font-semibold">{children}</strong>,
                  em: ({ children }) => <em className="italic">{children}</em>,
                  // Table components
                  table: ({ children }) => (
                    <div className="overflow-x-auto my-4 max-w-full">
                      <table className="w-full border-collapse text-xs sm:text-sm">
                        {children}
                      </table>
                    </div>
                  ),
                  thead: ({ children }) => <thead className="bg-muted/50">{children}</thead>,
                  tbody: ({ children }) => <tbody>{children}</tbody>,
                  tr: ({ children }) => <tr className="border-b border-border">{children}</tr>,
                  th: ({ children }) => (
                    <th className="px-1 sm:px-3 py-1 sm:py-2 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                      {children}
                    </th>
                  ),
                  td: ({ children }) => <td className="px-1 sm:px-3 py-1 sm:py-2 text-xs sm:text-sm break-words">{children}</td>,
                }}
              >
                {message.content}
              </ReactMarkdown>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}