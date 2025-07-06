'use client'

import { useEffect, useRef, useState } from 'react'
import { useChatStore } from '@/lib/store'
import { ChatMessage } from './chat-message'
import { ChatInput } from './chat-input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Card } from '@/components/ui/card'
import { FileText, Loader2, MessageSquare } from 'lucide-react'

export function ChatView() {
  const { getCurrentSession, addMessage, currentSessionId } = useChatStore()
  const [isLoading, setIsLoading] = useState(false)
  const [isHydrated, setIsHydrated] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const session = getCurrentSession()

  useEffect(() => {
    setIsHydrated(true)
  }, [])

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [session?.messages])

  const handleSendMessage = async (content: string) => {
    if (!currentSessionId) return

    // Add user message
    addMessage(currentSessionId, {
      role: 'user',
      content,
    })

    setIsLoading(true)
    try {
      const response = await fetch('/api/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          message: content,
          sessionId: currentSessionId,
        }),
      })

      if (!response.ok) {
        throw new Error('Failed to send message')
      }

      const data = await response.json()
      
      // Add assistant response
      addMessage(currentSessionId, {
        role: 'assistant',
        content: data.result || data.message || 'No response',
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
      
      // Add system message about document upload
      addMessage(currentSessionId, {
        role: 'system',
        content: `Document "${file.name}" uploaded successfully. ${data.chunks_created || 0} chunks created.`,
      })
    } catch (error) {
      console.error('Error uploading document:', error)
      addMessage(currentSessionId, {
        role: 'system',
        content: `Failed to upload document "${file.name}". Please try again.`,
      })
    }
  }

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
        <div className="pb-4">
          {session.messages.length === 0 ? (
            <div className="flex h-full items-center justify-center p-8 text-center text-muted-foreground">
              <div>
                <h3 className="mb-2 text-lg font-semibold">Welcome to Paladin AI</h3>
                <p className="mb-4">Start a conversation or upload a document to begin.</p>
                <div className="flex items-center justify-center gap-2 text-sm">
                  <FileText className="h-4 w-4" />
                  <span>Drop files here or click the paperclip to upload</span>
                </div>
              </div>
            </div>
          ) : (
            session.messages.map((message) => (
              <ChatMessage key={message.id} message={message} />
            ))
          )}
          {isLoading && (
            <div className="flex items-center justify-center p-4">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          )}
        </div>
      </ScrollArea>

      <ChatInput
        onSendMessage={handleSendMessage}
        onUploadDocument={handleUploadDocument}
        isLoading={isLoading}
      />
    </div>
  )
}