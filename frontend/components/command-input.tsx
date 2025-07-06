'use client'

import { useState, useRef, useEffect, KeyboardEvent } from 'react'
import { Send, Terminal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CommandHistory, getCommandSuggestions } from '@/lib/commands'

interface CommandInputProps {
  onCommand: (command: string) => void
  isLoading?: boolean
  placeholder?: string
}

export function CommandInput({ onCommand, isLoading = false, placeholder }: CommandInputProps) {
  const [input, setInput] = useState('')
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [selectedSuggestion, setSelectedSuggestion] = useState(0)
  const [showSuggestions, setShowSuggestions] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const historyRef = useRef(new CommandHistory())

  useEffect(() => {
    // Update suggestions when input changes
    if (input.startsWith('/')) {
      const newSuggestions = getCommandSuggestions(input)
      setSuggestions(newSuggestions)
      setShowSuggestions(newSuggestions.length > 0 && input.length > 1)
      setSelectedSuggestion(0)
    } else {
      setShowSuggestions(false)
    }
  }, [input])

  const handleSubmit = () => {
    if (input.trim() && !isLoading) {
      historyRef.current.add(input)
      onCommand(input)
      setInput('')
      setShowSuggestions(false)
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    // Command history navigation
    if (e.key === 'ArrowUp' && !showSuggestions) {
      e.preventDefault()
      const prev = historyRef.current.getPrevious()
      if (prev !== null) {
        setInput(prev)
      }
    } else if (e.key === 'ArrowDown' && !showSuggestions) {
      e.preventDefault()
      const next = historyRef.current.getNext()
      if (next !== null) {
        setInput(next)
      }
    }
    
    // Suggestion navigation
    else if (e.key === 'ArrowUp' && showSuggestions) {
      e.preventDefault()
      setSelectedSuggestion(prev => Math.max(0, prev - 1))
    } else if (e.key === 'ArrowDown' && showSuggestions) {
      e.preventDefault()
      setSelectedSuggestion(prev => Math.min(suggestions.length - 1, prev + 1))
    }
    
    // Accept suggestion
    else if (e.key === 'Tab' && showSuggestions && suggestions.length > 0) {
      e.preventDefault()
      setInput(suggestions[selectedSuggestion])
      setShowSuggestions(false)
    }
    
    // Submit on Enter
    else if (e.key === 'Enter') {
      if (showSuggestions && suggestions.length > 0) {
        e.preventDefault()
        setInput(suggestions[selectedSuggestion])
        setShowSuggestions(false)
      } else {
        e.preventDefault()
        handleSubmit()
      }
    }
    
    // Close suggestions on Escape
    else if (e.key === 'Escape' && showSuggestions) {
      e.preventDefault()
      setShowSuggestions(false)
    }
  }

  const getPlaceholder = () => {
    if (placeholder) return placeholder
    return "Type a message or '/' for commands..."
  }

  return (
    <div className="relative p-2 sm:p-4 border-t bg-background">
      {/* Suggestions dropdown */}
      {showSuggestions && (
        <div className="absolute bottom-full left-0 right-0 mb-1 bg-background border rounded-md shadow-lg max-h-48 overflow-auto z-10">
          {suggestions.map((suggestion, index) => (
            <div
              key={suggestion}
              className={`px-2 sm:px-3 py-1.5 sm:py-2 cursor-pointer hover:bg-muted ${
                index === selectedSuggestion ? 'bg-muted' : ''
              }`}
              onClick={() => {
                setInput(suggestion)
                setShowSuggestions(false)
                inputRef.current?.focus()
              }}
            >
              <div className="flex items-center gap-2">
                <Terminal className="h-3 w-3 text-muted-foreground" />
                <span className="font-mono text-xs sm:text-sm">{suggestion}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Input field */}
      <div className="flex gap-1 sm:gap-2">
        <div className="relative flex-1">
          <Input
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={getPlaceholder()}
            disabled={isLoading}
            className={`pr-8 sm:pr-10 text-sm ${input.startsWith('/') ? 'font-mono' : ''}`}
          />
          {input.startsWith('/') && (
            <Terminal className="absolute right-2 sm:right-3 top-1/2 -translate-y-1/2 h-3 w-3 sm:h-4 sm:w-4 text-muted-foreground" />
          )}
        </div>
        <Button
          onClick={handleSubmit}
          disabled={!input.trim() || isLoading}
          size="icon"
          className="h-8 w-8 sm:h-10 sm:w-10"
        >
          <Send className="h-3 w-3 sm:h-4 sm:w-4" />
        </Button>
      </div>

      {/* Help text */}
      {input === '/' && (
        <div className="absolute top-full left-0 mt-1 text-xs text-muted-foreground">
          Type /help to see available commands
        </div>
      )}
    </div>
  )
}