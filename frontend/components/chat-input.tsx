import { useState, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Send, Paperclip, Loader2 } from 'lucide-react'
import { useDropzone } from 'react-dropzone'
import { cn } from '@/lib/utils'

interface ChatInputProps {
  onSendMessage: (message: string) => void
  onUploadDocument: (file: File) => Promise<void>
  isLoading?: boolean
}

export function ChatInput({ onSendMessage, onUploadDocument, isLoading }: ChatInputProps) {
  const [input, setInput] = useState('')
  const [isUploading, setIsUploading] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (input.trim() && !isLoading) {
      onSendMessage(input.trim())
      setInput('')
    }
  }

  const handleFileUpload = async (files: File[]) => {
    if (files.length === 0) return
    
    setIsUploading(true)
    try {
      for (const file of files) {
        await onUploadDocument(file)
      }
    } finally {
      setIsUploading(false)
    }
  }

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop: handleFileUpload,
    noClick: true,
    noKeyboard: true,
  })

  return (
    <div {...getRootProps()} className="border-t bg-background p-4">
      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          {...getInputProps()}
          ref={fileInputRef}
          type="file"
          className="hidden"
          onChange={(e) => {
            const files = Array.from(e.target.files || [])
            handleFileUpload(files)
          }}
        />
        
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={() => fileInputRef.current?.click()}
          disabled={isUploading}
        >
          {isUploading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Paperclip className="h-4 w-4" />
          )}
        </Button>

        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={isDragActive ? "Drop files here..." : "Type a message..."}
          className={cn(
            "flex-1",
            isDragActive && "border-primary"
          )}
          disabled={isLoading || isUploading}
        />

        <Button 
          type="submit" 
          size="icon"
          disabled={!input.trim() || isLoading || isUploading}
        >
          {isLoading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Send className="h-4 w-4" />
          )}
        </Button>
      </form>
    </div>
  )
}