'use client'

import { AlertCircle, CheckCircle, Info, Terminal, AlertTriangle } from 'lucide-react'
import { Card } from '@/components/ui/card'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { CommandResult } from '@/lib/commands'

interface CommandMessageProps {
  command: string
  result: CommandResult
  timestamp: Date
}

export function CommandMessage({ command, result, timestamp }: CommandMessageProps) {
  const getIcon = () => {
    switch (result.type) {
      case 'success':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'error':
        return <AlertCircle className="h-4 w-4 text-red-500" />
      case 'warning':
        return <AlertTriangle className="h-4 w-4 text-yellow-500" />
      case 'info':
      default:
        return <Info className="h-4 w-4 text-blue-500" />
    }
  }

  const getBorderColor = () => {
    switch (result.type) {
      case 'success':
        return 'border-green-500/20'
      case 'error':
        return 'border-red-500/20'
      case 'warning':
        return 'border-yellow-500/20'
      case 'info':
      default:
        return 'border-blue-500/20'
    }
  }

  const formatContent = (content: string) => {
    // Check if content is JSON
    try {
      if (content.startsWith('{') || content.startsWith('[')) {
        const parsed = JSON.parse(content)
        return (
          <pre className="mt-2 overflow-auto">
            <code className="text-sm">{JSON.stringify(parsed, null, 2)}</code>
          </pre>
        )
      }
    } catch (e) {
      // Not JSON, continue with markdown
    }

    // Render as markdown
    return (
      <div className="prose prose-sm dark:prose-invert max-w-none">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            pre: ({ children }) => (
              <pre className="bg-muted p-3 rounded-md overflow-auto mt-2">
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
            h1: ({ children }) => <h1 className="text-base sm:text-lg font-bold mt-3 sm:mt-4 mb-2">{children}</h1>,
            h2: ({ children }) => <h2 className="text-sm sm:text-base font-semibold mt-2 sm:mt-3 mb-1">{children}</h2>,
            h3: ({ children }) => <h3 className="text-xs sm:text-sm font-semibold mt-2 mb-1">{children}</h3>,
            ul: ({ children }) => <ul className="list-disc list-inside ml-4 my-2">{children}</ul>,
            li: ({ children }) => <li className="my-0.5">{children}</li>,
            // Table components
            table: ({ children }) => (
              <div className="overflow-x-auto my-3">
                <table className="min-w-full border-collapse text-xs sm:text-sm">
                  {children}
                </table>
              </div>
            ),
            thead: ({ children }) => <thead className="bg-muted/50">{children}</thead>,
            tbody: ({ children }) => <tbody>{children}</tbody>,
            tr: ({ children }) => <tr className="border-b border-border">{children}</tr>,
            th: ({ children }) => (
              <th className="px-2 sm:px-3 py-1.5 sm:py-2 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                {children}
              </th>
            ),
            td: ({ children }) => <td className="px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm">{children}</td>,
          }}
        >
          {content}
        </ReactMarkdown>
      </div>
    )
  }

  return (
    <Card className={`p-3 sm:p-4 ${getBorderColor()} border-l-4`}>
      {/* Command header */}
      <div className="flex items-center gap-2 mb-2 text-xs sm:text-sm text-muted-foreground">
        <Terminal className="h-3 w-3 shrink-0" />
        <span className="font-mono truncate">{command}</span>
        <span className="ml-auto text-xs shrink-0">
          {timestamp.toLocaleTimeString()}
        </span>
      </div>

      {/* Result */}
      <div className="flex items-start gap-2">
        <div className="mt-0.5">{getIcon()}</div>
        <div className="flex-1 min-w-0">
          {formatContent(result.content)}
        </div>
      </div>
    </Card>
  )
}