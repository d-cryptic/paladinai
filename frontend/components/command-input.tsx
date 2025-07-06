'use client'

import { useState, useRef, useEffect, KeyboardEvent } from 'react'
import { Send, Terminal, Paperclip } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CommandHistory, getCommandSuggestions, COMMANDS } from '@/lib/commands'

// Command sub-options (duplicated here for Tab completion)
const COMMAND_SUB_OPTIONS: Record<string, string[]> = {
  memory: ['search', 'store', 'types', 'health'],
  checkpoint: ['get', 'exists', 'list', 'delete'],
  document: ['search', 'health'],
  alert: ['analyze'],
}

interface CommandInputProps {
  onCommand: (command: string) => void
  isLoading?: boolean
  placeholder?: string
  value?: string
  onChange?: (value: string) => void
  onFileUpload?: (file: File) => void
}

export function CommandInput({ onCommand, isLoading = false, placeholder, value, onChange, onFileUpload }: CommandInputProps) {
  const [localInput, setLocalInput] = useState('')
  const input = value !== undefined ? value : localInput
  const setInput = (newValue: string) => {
    if (onChange) {
      onChange(newValue)
    } else {
      setLocalInput(newValue)
    }
  }
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [selectedSuggestion, setSelectedSuggestion] = useState(0)
  const [showSuggestions, setShowSuggestions] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const historyRef = useRef(new CommandHistory())
  const suggestionsRef = useRef<HTMLDivElement>(null)
  const selectedItemRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Sync external value changes to local state
  useEffect(() => {
    if (value !== undefined) {
      setLocalInput(value)
    }
  }, [value])

  useEffect(() => {
    // Update suggestions when input changes
    if (input === '/') {
      // Show all commands when just typing /
      const allCommands = Object.keys(COMMANDS).map(cmd => `/${cmd}`)
      setSuggestions(allCommands)
      setShowSuggestions(true)
      setSelectedSuggestion(0)
    } else if (input.startsWith('/')) {
      const parts = input.slice(1).split(' ')
      const mainCommand = parts[0].toLowerCase()
      
      const newSuggestions = getCommandSuggestions(input)
      setSuggestions(newSuggestions)
      setShowSuggestions(newSuggestions.length > 0)
      setSelectedSuggestion(0)
    } else {
      setShowSuggestions(false)
    }
  }, [input])

  // Scroll selected suggestion into view
  useEffect(() => {
    if (selectedItemRef.current && suggestionsRef.current) {
      const container = suggestionsRef.current
      const element = selectedItemRef.current
      
      const elementTop = element.offsetTop
      const elementBottom = elementTop + element.offsetHeight
      const containerTop = container.scrollTop
      const containerBottom = containerTop + container.clientHeight
      
      if (elementTop < containerTop) {
        container.scrollTop = elementTop
      } else if (elementBottom > containerBottom) {
        container.scrollTop = elementBottom - container.clientHeight
      }
    }
  }, [selectedSuggestion])

  const handleSubmit = () => {
    if (input.trim() && !isLoading) {
      historyRef.current.add(input)
      onCommand(input)
      setInput('')
      setShowSuggestions(false)
      // Clear the external value if controlled
      if (onChange) {
        onChange('')
      }
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
      const selected = suggestions[selectedSuggestion]
      
      // If it's a command that has sub-options and doesn't already have a space, add one
      const cmdName = selected.slice(1).split(' ')[0]
      if (COMMAND_SUB_OPTIONS[cmdName] && !selected.includes(' ')) {
        setInput(selected + ' ')
        // Don't hide suggestions yet, let user see sub-options
      } else {
        setInput(selected)
        setShowSuggestions(false)
      }
    }
    
    // Submit on Enter
    else if (e.key === 'Enter') {
      if (showSuggestions && suggestions.length > 0) {
        e.preventDefault()
        const selected = suggestions[selectedSuggestion]
        
        // If it's a command that has sub-options and doesn't already have a space, add one
        const cmdName = selected.slice(1).split(' ')[0]
        if (COMMAND_SUB_OPTIONS[cmdName] && !selected.includes(' ')) {
          setInput(selected + ' ')
          // Don't hide suggestions yet, let user see sub-options
        } else {
          setInput(selected)
          setShowSuggestions(false)
        }
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

  const handleFileUpload = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file && onFileUpload) {
      onFileUpload(file)
      // Reset the input so the same file can be uploaded again
      event.target.value = ''
    }
  }

  const getPlaceholder = () => {
    if (placeholder) return placeholder
    return "Type a message or '/' for commands..."
  }

  return (
    <div className="relative p-3 sm:p-4 lg:p-6 border-t bg-background shrink-0">
      <div className="xl:max-w-5xl xl:mx-auto xl:px-8 max-w-full overflow-hidden">
        {/* Suggestions dropdown */}
        {showSuggestions && (
          <div 
            ref={suggestionsRef}
            className="absolute bottom-full left-0 right-0 mb-1 bg-background border rounded-md shadow-lg max-h-64 overflow-y-auto overflow-x-hidden z-10 scrollbar-thin scrollbar-thumb-muted scrollbar-track-transparent"
          >
          {suggestions.map((suggestion, index) => {
            const cmdName = suggestion.slice(1).split(' ')[0]
            const cmd = COMMANDS[cmdName]
            const isSelected = index === selectedSuggestion
            return (
              <div
                key={suggestion}
                ref={isSelected ? selectedItemRef : null}
                className={`px-2 sm:px-3 py-2 sm:py-3 cursor-pointer transition-colors ${
                  isSelected ? 'bg-muted' : 'hover:bg-muted/50'
                }`}
                onMouseEnter={() => setSelectedSuggestion(index)}
                onClick={() => {
                  const cmdName = suggestion.slice(1).split(' ')[0]
                  if (COMMAND_SUB_OPTIONS[cmdName] && !suggestion.includes(' ')) {
                    setInput(suggestion + ' ')
                    // Don't hide suggestions yet, let user see sub-options
                  } else {
                    setInput(suggestion)
                    setShowSuggestions(false)
                  }
                  inputRef.current?.focus()
                }}
              >
                <div className="flex items-start gap-2">
                  <Terminal className="h-3 w-3 text-muted-foreground mt-0.5 shrink-0" />
                  <div className="flex-1">
                    <div className="font-mono text-xs sm:text-sm font-medium">{suggestion}</div>
                    {cmd && (
                      <div className="text-xs text-muted-foreground mt-0.5">
                        {cmd.description}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        className="hidden"
        onChange={handleFileChange}
        accept=".pdf,.doc,.docx,.txt,.md,.json,.xml,.csv,.log"
      />

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
          
          {/* File upload button */}
          {onFileUpload && (
            <Button
              onClick={handleFileUpload}
              disabled={isLoading}
              size="icon"
              variant="outline"
              className="h-8 w-8 sm:h-10 sm:w-10"
              title="Upload document"
            >
              <Paperclip className="h-3 w-3 sm:h-4 sm:w-4" />
            </Button>
          )}
          
          <Button
            onClick={handleSubmit}
            disabled={!input.trim() || isLoading}
            size="icon"
            className="h-8 w-8 sm:h-10 sm:w-10"
          >
            <Send className="h-3 w-3 sm:h-4 sm:w-4" />
          </Button>
        </div>
      </div>
    </div>
  )
}