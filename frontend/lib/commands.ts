/**
 * Command parsing and execution logic for Paladin AI web interface
 */

export interface CommandResult {
  type: 'success' | 'error' | 'info' | 'warning'
  content: string
  data?: any
}

export interface Command {
  name: string
  description: string
  usage: string
  category: 'connection' | 'chat' | 'memory' | 'checkpoint' | 'document' | 'system'
  handler: (args: string[]) => Promise<CommandResult>
}

// Command categories for help display
export const COMMAND_CATEGORIES = {
  connection: 'Connection & Status',
  chat: 'Chat & Analysis',
  memory: 'Memory Management',
  checkpoint: 'Checkpoint Management',
  document: 'Document Management',
  system: 'System Commands'
}

// Parse command string into command name and arguments
export function parseCommand(input: string): { command: string; args: string[] } {
  const trimmed = input.trim()
  
  // Handle slash commands
  if (trimmed.startsWith('/')) {
    const parts = trimmed.slice(1).split(/\s+/)
    return {
      command: parts[0] || '',
      args: parts.slice(1)
    }
  }
  
  // Regular chat message
  return {
    command: 'chat',
    args: [trimmed]
  }
}

// Format API response for display
export function formatResponse(response: any): string {
  if (typeof response === 'string') {
    return response
  }
  
  // Handle structured responses
  if (response.result) {
    return response.result
  }
  
  if (response.message) {
    return response.message
  }
  
  if (response.status) {
    return `Status: ${response.status}`
  }
  
  // Format JSON responses nicely
  return JSON.stringify(response, null, 2)
}

// Execute API call to Paladin server
export async function executeAPICall(
  endpoint: string,
  method: 'GET' | 'POST' | 'DELETE' = 'GET',
  body?: any
): Promise<any> {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000'
  
  const options: RequestInit = {
    method,
    headers: {
      'Content-Type': 'application/json',
    },
  }
  
  if (body) {
    options.body = JSON.stringify(body)
  }
  
  const response = await fetch(`${apiUrl}${endpoint}`, options)
  
  if (!response.ok) {
    const errorText = await response.text()
    throw new Error(errorText || `HTTP ${response.status}`)
  }
  
  const contentType = response.headers.get('content-type')
  if (contentType && contentType.includes('application/json')) {
    return await response.json()
  }
  
  return await response.text()
}

// Command implementations
export const COMMANDS: Record<string, Command> = {
  help: {
    name: 'help',
    description: 'Show available commands',
    usage: '/help [command]',
    category: 'system',
    handler: async (args) => {
      if (args.length > 0) {
        const cmd = COMMANDS[args[0]]
        if (cmd) {
          return {
            type: 'info',
            content: `**${cmd.name}** - ${cmd.description}\n\nUsage: ${cmd.usage}`
          }
        }
        return {
          type: 'error',
          content: `Unknown command: ${args[0]}`
        }
      }
      
      // Show all commands grouped by category
      let helpText = '# Available Commands\n\n'
      
      for (const [category, title] of Object.entries(COMMAND_CATEGORIES)) {
        const commands = Object.values(COMMANDS).filter(cmd => cmd.category === category)
        if (commands.length === 0) continue
        
        helpText += `## ${title}\n`
        for (const cmd of commands) {
          helpText += `- **${cmd.usage}** - ${cmd.description}\n`
        }
        helpText += '\n'
      }
      
      return {
        type: 'info',
        content: helpText
      }
    }
  },
  
  // Connection commands
  test: {
    name: 'test',
    description: 'Test server connection',
    usage: '/test',
    category: 'connection',
    handler: async () => {
      try {
        const result = await executeAPICall('/health')
        return {
          type: 'success',
          content: '✅ Server connection successful!'
        }
      } catch (error) {
        return {
          type: 'error',
          content: `❌ Connection failed: ${error}`
        }
      }
    }
  },
  
  status: {
    name: 'status',
    description: 'Get API status',
    usage: '/status',
    category: 'connection',
    handler: async () => {
      try {
        const result = await executeAPICall('/api/v1/status')
        return {
          type: 'info',
          content: formatResponse(result)
        }
      } catch (error) {
        return {
          type: 'error',
          content: `Failed to get status: ${error}`
        }
      }
    }
  },
  
  // Memory commands
  memory: {
    name: 'memory',
    description: 'Memory management commands',
    usage: '/memory <action> [args]',
    category: 'memory',
    handler: async (args) => {
      if (args.length === 0) {
        return {
          type: 'info',
          content: `Memory commands:
- /memory search <query> - Search memories
- /memory store <instruction> - Store instruction
- /memory types - Show memory types
- /memory health - Check memory health`
        }
      }
      
      const action = args[0].toLowerCase()
      const remainingArgs = args.slice(1)
      
      switch (action) {
        case 'search':
          if (remainingArgs.length === 0) {
            return { type: 'error', content: 'Please provide a search query' }
          }
          try {
            const result = await executeAPICall('/api/memory/search', 'POST', {
              query: remainingArgs.join(' '),
              limit: 5
            })
            return {
              type: 'success',
              content: formatResponse(result)
            }
          } catch (error) {
            return { type: 'error', content: `Search failed: ${error}` }
          }
          
        case 'store':
          if (remainingArgs.length === 0) {
            return { type: 'error', content: 'Please provide an instruction to store' }
          }
          try {
            const result = await executeAPICall('/api/memory/instruction', 'POST', {
              instruction: remainingArgs.join(' ')
            })
            return {
              type: 'success',
              content: '✅ Instruction stored successfully'
            }
          } catch (error) {
            return { type: 'error', content: `Store failed: ${error}` }
          }
          
        case 'types':
          try {
            const result = await executeAPICall('/api/memory/types')
            return {
              type: 'info',
              content: formatResponse(result)
            }
          } catch (error) {
            return { type: 'error', content: `Failed to get memory types: ${error}` }
          }
          
        case 'health':
          try {
            const result = await executeAPICall('/api/memory/health')
            return {
              type: 'success',
              content: '✅ Memory service is healthy'
            }
          } catch (error) {
            return { type: 'error', content: `Memory service health check failed: ${error}` }
          }
          
        default:
          return { type: 'error', content: `Unknown memory action: ${action}` }
      }
    }
  },
  
  // Checkpoint commands
  checkpoint: {
    name: 'checkpoint',
    description: 'Checkpoint management commands',
    usage: '/checkpoint <action> [args]',
    category: 'checkpoint',
    handler: async (args) => {
      if (args.length === 0) {
        return {
          type: 'info',
          content: `Checkpoint commands:
- /checkpoint get <session_id> - Get checkpoint
- /checkpoint exists <session_id> - Check if checkpoint exists
- /checkpoint list - List all checkpoints
- /checkpoint delete <session_id> - Delete checkpoint`
        }
      }
      
      const action = args[0].toLowerCase()
      const sessionId = args[1]
      
      switch (action) {
        case 'get':
          if (!sessionId) {
            return { type: 'error', content: 'Please provide a session ID' }
          }
          try {
            const result = await executeAPICall(`/api/v1/checkpoints/${sessionId}`)
            return {
              type: 'success',
              content: formatResponse(result)
            }
          } catch (error) {
            return { type: 'error', content: `Failed to get checkpoint: ${error}` }
          }
          
        case 'exists':
          if (!sessionId) {
            return { type: 'error', content: 'Please provide a session ID' }
          }
          try {
            const result = await executeAPICall(`/api/v1/checkpoints/${sessionId}/exists`)
            return {
              type: 'info',
              content: result.exists ? '✅ Checkpoint exists' : '❌ Checkpoint not found'
            }
          } catch (error) {
            return { type: 'error', content: `Failed to check checkpoint: ${error}` }
          }
          
        case 'list':
          try {
            const result = await executeAPICall('/api/v1/checkpoints/')
            return {
              type: 'success',
              content: formatResponse(result)
            }
          } catch (error) {
            return { type: 'error', content: `Failed to list checkpoints: ${error}` }
          }
          
        case 'delete':
          if (!sessionId) {
            return { type: 'error', content: 'Please provide a session ID' }
          }
          try {
            await executeAPICall(`/api/v1/checkpoints/${sessionId}`, 'DELETE')
            return {
              type: 'success',
              content: '✅ Checkpoint deleted successfully'
            }
          } catch (error) {
            return { type: 'error', content: `Failed to delete checkpoint: ${error}` }
          }
          
        default:
          return { type: 'error', content: `Unknown checkpoint action: ${action}` }
      }
    }
  },
  
  // Document commands
  document: {
    name: 'document',
    description: 'Document management commands',
    usage: '/document <action> [args]',
    category: 'document',
    handler: async (args) => {
      if (args.length === 0) {
        return {
          type: 'info',
          content: `Document commands:
- /document search <query> - Search documents
- /document health - Check document service health`
        }
      }
      
      const action = args[0].toLowerCase()
      const query = args.slice(1).join(' ')
      
      switch (action) {
        case 'search':
          if (!query) {
            return { type: 'error', content: 'Please provide a search query' }
          }
          try {
            const result = await executeAPICall('/api/v1/documents/search', 'POST', {
              query,
              limit: 5
            })
            return {
              type: 'success',
              content: formatResponse(result)
            }
          } catch (error) {
            return { type: 'error', content: `Search failed: ${error}` }
          }
          
        case 'health':
          try {
            const result = await executeAPICall('/api/v1/documents/health')
            return {
              type: 'success',
              content: '✅ Document service is healthy'
            }
          } catch (error) {
            return { type: 'error', content: `Document service health check failed: ${error}` }
          }
          
        default:
          return { type: 'error', content: `Unknown document action: ${action}` }
      }
    }
  },
  
  // System commands
  clear: {
    name: 'clear',
    description: 'Clear the chat',
    usage: '/clear',
    category: 'system',
    handler: async () => {
      return {
        type: 'info',
        content: 'CLEAR_CHAT',
        data: { action: 'clear' }
      }
    }
  },
  
  // Alert analysis mode
  alert: {
    name: 'alert',
    description: 'Analyze alerts from webhook server',
    usage: '/alert analyze',
    category: 'chat',
    handler: async (args) => {
      if (args.length === 0 || args[0].toLowerCase() !== 'analyze') {
        return {
          type: 'info',
          content: 'Usage: /alert analyze - Analyze pending alerts from webhook server'
        }
      }
      
      try {
        const result = await executeAPICall('/api/v1/alert-analysis-mode', 'POST', {
          mode: 'analysis'
        })
        return {
          type: 'success',
          content: formatResponse(result)
        }
      } catch (error) {
        return { type: 'error', content: `Alert analysis failed: ${error}` }
      }
    }
  }
}

// Command history management
export class CommandHistory {
  private history: string[] = []
  private currentIndex: number = -1
  private maxHistory: number = 100
  
  add(command: string) {
    // Don't add duplicates of the last command
    if (this.history.length > 0 && this.history[this.history.length - 1] === command) {
      return
    }
    
    this.history.push(command)
    if (this.history.length > this.maxHistory) {
      this.history.shift()
    }
    this.currentIndex = this.history.length
  }
  
  getPrevious(): string | null {
    if (this.history.length === 0) return null
    
    if (this.currentIndex > 0) {
      this.currentIndex--
    }
    return this.history[this.currentIndex] || null
  }
  
  getNext(): string | null {
    if (this.currentIndex < this.history.length - 1) {
      this.currentIndex++
      return this.history[this.currentIndex]
    }
    
    this.currentIndex = this.history.length
    return ''
  }
}

// Get command suggestions based on partial input
export function getCommandSuggestions(input: string): string[] {
  if (!input.startsWith('/')) return []
  
  const partial = input.slice(1).toLowerCase()
  const suggestions: string[] = []
  
  for (const [name, cmd] of Object.entries(COMMANDS)) {
    if (name.toLowerCase().startsWith(partial)) {
      suggestions.push(`/${name}`)
    }
  }
  
  return suggestions.sort()
}