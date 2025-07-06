'use client'

import { useEffect, useRef, useState } from 'react'
import { useChatStore } from '@/lib/store'
import { ChatMessage } from './chat-message'
import { CommandInput } from './command-input'
import { CommandMessage } from './command-message'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card } from '@/components/ui/card'
import { FileText, Loader2, MessageSquare } from 'lucide-react'
import { parseCommand, COMMANDS, CommandResult } from '@/lib/commands'
import { getContextualMemories, formatMemories, determineWorkflowType } from '@/lib/memory'
import { useDropzone } from 'react-dropzone'

export function ChatView() {
  const { getCurrentSession, addMessage, currentSessionId, clearSession } = useChatStore()
  const [isLoading, setIsLoading] = useState(false)
  const [isHydrated, setIsHydrated] = useState(false)
  const [commandResults, setCommandResults] = useState<Array<{ command: string; result: CommandResult; timestamp: Date; id: string }>>([])
  const scrollRef = useRef<HTMLDivElement>(null)
  const session = getCurrentSession()

  useEffect(() => {
    setIsHydrated(true)
  }, [])

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [session?.messages, commandResults])

  const handleCommand = async (input: string) => {
    if (!currentSessionId) return

    const { command, args } = parseCommand(input)
    
    // Handle regular chat messages
    if (command === 'chat') {
      handleSendMessage(args[0])
      return
    }
    
    // Handle CLI commands
    const cmd = COMMANDS[command]
    if (!cmd) {
      const result: CommandResult = {
        type: 'error',
        content: `Unknown command: ${command}. Type /help for available commands.`
      }
      setCommandResults(prev => [...prev, {
        command: input,
        result,
        timestamp: new Date(),
        id: `cmd_${Date.now()}`
      }])
      return
    }
    
    setIsLoading(true)
    try {
      const result = await cmd.handler(args)
      
      // Handle special actions
      if (result.data?.action === 'clear') {
        if (currentSessionId) {
          clearSession(currentSessionId)
          setCommandResults([])
        }
        return
      }
      
      setCommandResults(prev => [...prev, {
        command: input,
        result,
        timestamp: new Date(),
        id: `cmd_${Date.now()}`
      }])
    } catch (error) {
      const result: CommandResult = {
        type: 'error',
        content: `Command failed: ${error}`
      }
      setCommandResults(prev => [...prev, {
        command: input,
        result,
        timestamp: new Date(),
        id: `cmd_${Date.now()}`
      }])
    } finally {
      setIsLoading(false)
    }
  }

  const handleSendMessage = async (content: string) => {
    if (!currentSessionId) return

    // Add user message
    addMessage(currentSessionId, {
      role: 'user',
      content,
    })

    setIsLoading(true)
    try {
      // Fetch contextual memories first (like CLI does)
      const workflowType = determineWorkflowType(content)
      
      try {
        const memoryResponse = await getContextualMemories(content, workflowType, 2)
        
        if (memoryResponse && memoryResponse.success && memoryResponse.memories && memoryResponse.memories.length > 0) {
          // Show memories to user
          const memoryContent = formatMemories(memoryResponse.memories)
          addMessage(currentSessionId, {
            role: 'system',
            content: memoryContent,
          })
        } else {
          // Show no memories found message (optional, CLI shows this)
          console.log('No contextual memories found')
        }
      } catch (memoryError) {
        console.error('Failed to fetch memories:', memoryError)
        // Continue without memories - don't block the chat
      }
      // Create a unique session ID for each message (like CLI does)
      // This prevents checkpoint state issues on the server
      const messageSessionId = `chat_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
      
      // Log the request for debugging
      const requestPayload = {
        message: content,
        sessionId: messageSessionId,  // Use unique session ID per message
        // Include context like CLI does
        additional_context: {
          session_id: messageSessionId,  // Same unique ID here
          workflow_type: workflowType,
          ui_source: 'web',
          ui_chat_session: currentSessionId  // Keep track of UI session for grouping
        }
      }
      
      console.log('Sending chat request:', requestPayload)
      
      const response = await fetch('/api/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestPayload),
      })

      if (!response.ok) {
        const errorText = await response.text()
        console.error('Chat API error:', errorText)
        throw new Error(errorText || 'Failed to send message')
      }

      const data = await response.json()
      
      console.log('Chat response:', data)
      
      // Extract formatted content like the CLI does
      let formattedContent = ''
      
      // First priority: Check for markdown content in the 'content' field
      if (data.content && data.content.length > 50) {
        formattedContent = data.content
      } 
      // Second priority: Check for formatted markdown in raw_result
      else if (data.raw_result?.formatted_markdown) {
        formattedContent = data.raw_result.formatted_markdown
      }
      // Third priority: Use the simple response from query/action/incident results
      else if (data.raw_result?.query_result?.response) {
        formattedContent = data.raw_result.query_result.response
      } else if (data.raw_result?.action_result?.response) {
        formattedContent = data.raw_result.action_result.response
      } else if (data.raw_result?.incident_result?.response) {
        formattedContent = data.raw_result.incident_result.response
      }
      // Fallback: Show a formatted version of the raw data
      else {
        formattedContent = '```json\n' + JSON.stringify(data, null, 2) + '\n```'
      }
      
      // Add metadata footer if available (like CLI does)
      if (data.metadata) {
        formattedContent += '\n\n---\n'
        formattedContent += `*Session: ${data.session_id || 'N/A'} | `
        formattedContent += `Type: ${data.metadata.workflow_type || 'N/A'} | `
        formattedContent += `Time: ${data.metadata.execution_time_ms || 0}ms*`
      }
      
      // Add assistant response
      addMessage(currentSessionId, {
        role: 'assistant',
        content: formattedContent || 'No response received',
      })
    } catch (error) {
      console.error('Error sending message:', error)
      addMessage(currentSessionId, {
        role: 'assistant',
        content: 'Sorry, I encountered an error. Please try again.',
      })
    } finally {
      setIsLoading(false)
    }
  }

  const handleUploadDocument = async (file: File) => {
    if (!currentSessionId) return

    const formData = new FormData()
    formData.append('file', file)

    try {
      const response = await fetch('/api/documents', {
        method: 'POST',
        body: formData,
      })

      if (!response.ok) {
        throw new Error('Failed to upload document')
      }

      const data = await response.json()
      
      // Show success as command result
      const result: CommandResult = {
        type: 'success',
        content: `Document "${file.name}" uploaded successfully! ${data.chunks_created || 0} chunks created.`
      }
      setCommandResults(prev => [...prev, {
        command: `Upload: ${file.name}`,
        result,
        timestamp: new Date(),
        id: `upload_${Date.now()}`
      }])
    } catch (error) {
      console.error('Error uploading document:', error)
      
      const result: CommandResult = {
        type: 'error',
        content: `Failed to upload document "${file.name}". Please try again.`
      }
      setCommandResults(prev => [...prev, {
        command: `Upload: ${file.name}`,
        result,
        timestamp: new Date(),
        id: `upload_${Date.now()}`
      }])
    }
  }
  
  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop: (acceptedFiles) => {
      acceptedFiles.forEach(file => handleUploadDocument(file))
    },
    noClick: true,
    noKeyboard: true,
  })

  if (!isHydrated) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <Loader2 className="h-8 w-8 animate-spin" />
      </div>
    )
  }

  if (!session) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        <div className="text-center">
          <MessageSquare className="mx-auto h-12 w-12 mb-4" />
          <p>Select or create a chat to get started</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      <div className="border-b p-4">
        <h2 className="font-semibold">{session.title}</h2>
      </div>

      <ScrollArea className="flex-1" ref={scrollRef}>
        <div className="pb-4" {...getRootProps()}>
          <input {...getInputProps()} />
          {session.messages.length === 0 && commandResults.length === 0 ? (
            <div className="flex h-full items-center justify-center p-8 text-center text-muted-foreground">
              <div>
                <h3 className="mb-2 text-lg font-semibold">Welcome to Paladin AI</h3>
                <p className="mb-4">Start a conversation or use CLI commands to interact with the server.</p>
                <div className="space-y-2 text-sm">
                  <div className="flex items-center justify-center gap-2">
                    <FileText className="h-4 w-4" />
                    <span>Drop files here to upload documents</span>
                  </div>
                  <div className="text-xs">
                    Type <code className="bg-muted px-1 py-0.5 rounded">/help</code> to see available commands
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-4 p-4">
              {/* Command results */}
              {commandResults.map((result) => (
                <CommandMessage
                  key={result.id}
                  command={result.command}
                  result={result.result}
                  timestamp={result.timestamp}
                />
              ))}
              
              {/* Chat messages */}
              {session.messages.map((message) => (
                <ChatMessage key={message.id} message={message} />
              ))}
            </div>
          )}
          {isLoading && (
            <div className="flex items-center justify-center p-4">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          )}
          {isDragActive && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
              <div className="text-center">
                <FileText className="mx-auto h-12 w-12 text-primary" />
                <p className="mt-2 text-lg font-medium">Drop files to upload</p>
              </div>
            </div>
          )}
        </div>
      </ScrollArea>

      <CommandInput
        onCommand={handleCommand}
        isLoading={isLoading}
        placeholder={isDragActive ? "Drop files here..." : undefined}
      />
    </div>
  )
}