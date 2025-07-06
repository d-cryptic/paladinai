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
  
  // Handle memory search results
  if (response.success && response.memories && Array.isArray(response.memories)) {
    let output = `# 🧠 Memory Search Results\n\n`
    output += `🔍 **Query:** \`${response.query}\`\n`
    output += `📊 **Total Results:** ${response.total_results}\n\n`
    
    if (response.memories.length === 0) {
      output += `*No memories found matching your query.*\n\n`
      output += `**💡 Tips:**\n`
      output += `- Try different keywords\n`
      output += `- Use \`/memory store <instruction>\` to add relevant memories\n`
      return output
    }
    
    output += `---\n\n`
    
    response.memories.forEach((memory: any, index: number) => {
      const score = (memory.score * 100).toFixed(1)
      const scoreIcon = memory.score >= 0.8 ? '🟢' : memory.score >= 0.6 ? '🟡' : '🔴'
      
      output += `### ${index + 1}. ${scoreIcon} ${memory.memory_type.charAt(0).toUpperCase() + memory.memory_type.slice(1)} Memory\n\n`
      
      output += `**📈 Score:** ${score}%\n\n`
      
      output += `**📝 Content:**\n`
      output += `> ${memory.memory}\n\n`
      
      output += `**📅 Created:** ${new Date(memory.created_at).toLocaleString()}\n\n`
      
      if (memory.related_entities && memory.related_entities.length > 0) {
        output += `**🏷️ Related Entities:**\n`
        const entities = memory.related_entities.map((e: any) => `\`${e.entity}\``).join(', ')
        output += `${entities}\n\n`
      }
      
      // Separator between results
      if (index < response.memories.length - 1) {
        output += `---\n\n`
      }
    })
    
    // Footer with usage tips
    output += `\n**💡 Pro Tips:**\n`
    output += `- Higher scores (🟢) indicate better matches\n`
    output += `- Use \`/memory store <instruction>\` to add new memories\n`
    output += `- Try \`/memory types\` to see available memory categories\n`
    
    return output
  }
  
  // Handle memory types response
  if (response.memory_types && response.examples) {
    let output = `# Memory Types\n\n`
    output += `**System:** ${response.memory_types}\n\n`
    
    if (response.descriptions) {
      output += `## Available Types\n\n`
      Object.entries(response.descriptions).forEach(([type, desc]: [string, any]) => {
        output += `- **${type}**: ${desc}\n`
      })
      output += `\n`
    }
    
    if (response.examples) {
      output += `## Examples\n`
      response.examples.forEach((example: string) => {
        output += `- ${example}\n`
      })
    }
    
    return output
  }
  
  // Handle checkpoint list response
  if (Array.isArray(response) && response.length > 0 && response[0].session_id) {
    let output = `# Checkpoint List\n\n`
    response.forEach((checkpoint: any, index: number) => {
      output += `## ${index + 1}. Session: ${checkpoint.session_id}\n`
      output += `**Created:** ${new Date(checkpoint.created_at).toLocaleString()}\n`
      if (checkpoint.updated_at) {
        output += `**Updated:** ${new Date(checkpoint.updated_at).toLocaleString()}\n`
      }
      if (checkpoint.status) {
        output += `**Status:** ${checkpoint.status}\n`
      }
      output += `\n`
    })
    return output
  }
  
  // Handle single checkpoint get response (data directly in response.data)
  if (response.data && response.data.thread_id && response.data.checkpoint_id && !Array.isArray(response.data)) {
    const checkpoint = response.data
    const sizeFormatted = checkpoint.size_mb > 0 ? `${checkpoint.size_mb} MB` : `${(checkpoint.size_bytes / 1024).toFixed(1)} KB`
    const stepIcon = checkpoint.metadata?.step >= 8 ? '✅' : '🔄'
    
    let output = `# 🔍 Checkpoint Details\n\n`
    output += `### ${stepIcon} Checkpoint Information\n\n`
    output += `| Field | Value |\n`
    output += `|-------|-------|\n`
    output += `| **Thread ID** | \`${checkpoint.thread_id}\` |\n`
    output += `| **Checkpoint ID** | \`${checkpoint.checkpoint_id}\` |\n`
    output += `| **Created** | ${new Date(checkpoint.timestamp).toLocaleString()} |\n`
    output += `| **Size** | ${sizeFormatted} (${checkpoint.size_bytes.toLocaleString()} bytes) |\n`
    
    if (checkpoint.metadata) {
      output += `| **Source** | ${checkpoint.metadata.source?.replace(/"/g, '') || 'N/A'} |\n`
      if (checkpoint.metadata.step !== undefined) {
        output += `| **Workflow Step** | ${checkpoint.metadata.step}/9 |\n`
      }
    }
    
    // Show workflow execution details if available
    if (checkpoint.metadata && checkpoint.metadata.writes && checkpoint.metadata.writes.result) {
      const result = checkpoint.metadata.writes.result
      output += `\n### 🚀 Workflow Execution Details\n\n`
      
      if (result.user_input) {
        output += `**Original Request:** ${result.user_input.replace(/"/g, '')}\n\n`
      }
      
      if (result.current_node && result.current_phase) {
        output += `**Status:** ${result.current_phase.replace(/"/g, '')} (Node: ${result.current_node.replace(/"/g, '')})\n\n`
      }
      
      if (result.execution_path) {
        try {
          const path = JSON.parse(result.execution_path)
          output += `**Execution Path:** ${path.join(' → ')}\n\n`
        } catch (e) {
          output += `**Execution Path:** ${result.execution_path}\n\n`
        }
      }
      
      if (result.execution_time_ms) {
        const timeMs = parseInt(result.execution_time_ms)
        const timeSec = (timeMs / 1000).toFixed(1)
        output += `**Execution Time:** ${timeSec}s (${timeMs.toLocaleString()}ms)\n\n`
      }
      
      // Show final result summary if available
      if (result.final_result) {
        output += `### 📊 Result Summary\n\n`
        const finalResult = result.final_result
        
        if (finalResult.categorization) {
          output += `**Workflow Type:** ${finalResult.categorization.workflow_type?.replace(/"/g, '') || 'N/A'}\n`
          output += `**Confidence:** ${finalResult.categorization.confidence?.replace(/"/g, '') || 'N/A'}\n\n`
        }
        
        if (finalResult.action_result) {
          const actionResult = finalResult.action_result
          
          if (actionResult.data_characteristics) {
            output += `**Data Collected:**\n`
            output += `- Source: ${actionResult.data_characteristics.data_source?.replace(/"/g, '') || 'N/A'}\n`
            output += `- Quality: ${actionResult.data_characteristics.data_quality?.replace(/"/g, '') || 'N/A'}\n`
            output += `- Data Points: ${actionResult.data_characteristics.total_data_points || 'N/A'}\n\n`
          }
          
          if (actionResult.performance_assessment) {
            output += `**Performance Summary:**\n`
            const perf = actionResult.performance_assessment
            if (perf.cpu_performance) {
              output += `- CPU: ${perf.cpu_performance.average_usage?.replace(/"/g, '') || 'N/A'} (${perf.cpu_performance.status?.replace(/"/g, '') || 'N/A'})\n`
            }
            if (perf.memory_performance) {
              output += `- Memory: ${perf.memory_performance.average_usage?.replace(/"/g, '') || 'N/A'} (${perf.memory_performance.status?.replace(/"/g, '') || 'N/A'})\n`
            }
            output += `\n`
          }
          
          if (actionResult.key_insights) {
            try {
              const insights = JSON.parse(actionResult.key_insights)
              output += `**Key Insights:**\n`
              insights.forEach((insight: string, index: number) => {
                output += `${index + 1}. ${insight}\n`
              })
              output += `\n`
            } catch (e) {
              output += `**Key Insights:** ${actionResult.key_insights}\n\n`
            }
          }
        }
        
        if (finalResult.instruction_compliance) {
          const compliance = finalResult.instruction_compliance
          const score = parseFloat(compliance.compliance_score || '0')
          const percentage = (score * 100).toFixed(1)
          output += `**Instruction Compliance:** ${percentage}% (${compliance.instructions_provided || 0} provided)\n\n`
        }
      }
    }
    
    return output
  }
  
  // Handle checkpoint response with details
  if (response.data && response.data.checkpoints && Array.isArray(response.data.checkpoints)) {
    let output = `# 📋 Checkpoint List\n\n`
    output += `**Total Found:** ${response.data.count || response.data.checkpoints.length} checkpoints\n\n`
    
    response.data.checkpoints.forEach((checkpoint: any, index: number) => {
      const stepIcon = checkpoint.metadata?.step >= 8 ? '✅' : '🔄'
      const sizeFormatted = checkpoint.size_mb > 0 ? `${checkpoint.size_mb} MB` : `${(checkpoint.size_bytes / 1024).toFixed(1)} KB`
      
      output += `---\n\n`
      output += `### ${stepIcon} Checkpoint #${index + 1}\n\n`
      output += `| Field | Value |\n`
      output += `|-------|-------|\n`
      output += `| **Thread ID** | \`${checkpoint.thread_id}\` |\n`
      output += `| **Checkpoint ID** | \`${checkpoint.checkpoint_id}\` |\n`
      output += `| **Created** | ${new Date(checkpoint.timestamp).toLocaleString()} |\n`
      output += `| **Size** | ${sizeFormatted} (${checkpoint.size_bytes.toLocaleString()} bytes) |\n`
      if (checkpoint.metadata?.source) {
        output += `| **Source** | ${checkpoint.metadata.source.replace(/"/g, '')} |\n`
      }
      if (checkpoint.metadata?.step !== undefined) {
        output += `| **Workflow Step** | ${checkpoint.metadata.step}/9 |\n`
      }
      output += `\n`
    })
    return output
  }
  
  // Handle checkpoint response with details (fallback for direct checkpoints array)
  if (response.checkpoints && Array.isArray(response.checkpoints)) {
    let output = `# Checkpoint List\n\n`
    output += `**Total:** ${response.checkpoints.length}\n\n`
    response.checkpoints.forEach((checkpoint: any, index: number) => {
      output += `## ${index + 1}. Session: ${checkpoint.session_id || checkpoint.id}\n`
      output += `**Created:** ${new Date(checkpoint.created_at).toLocaleString()}\n`
      if (checkpoint.updated_at) {
        output += `**Updated:** ${new Date(checkpoint.updated_at).toLocaleString()}\n`
      }
      if (checkpoint.status) {
        output += `**Status:** ${checkpoint.status}\n`
      }
      if (checkpoint.workflow_state) {
        output += `**Workflow State:** ${checkpoint.workflow_state}\n`
      }
      output += `\n`
    })
    return output
  }
  
  // Handle document search results
  if (response.results && Array.isArray(response.results)) {
    let output = `# 📄 Document Search Results\n\n`
    
    // Header info
    if (response.query) {
      output += `🔍 **Query:** \`${response.query}\`\n`
    }
    
    const totalResults = response.count || response.total_results || response.results.length
    output += `📊 **Found:** ${totalResults} result${totalResults !== 1 ? 's' : ''}\n\n`
    
    if (response.results.length === 0) {
      output += `*No documents found matching your query.*\n\n`
      output += `**💡 Tips:**\n`
      output += `- Try different keywords\n`
      output += `- Use broader search terms\n`
      output += `- Check if documents have been uploaded\n`
      return output
    }
    
    output += `---\n\n`
    
    response.results.forEach((doc: any, index: number) => {
      const relevanceScore = doc.score ? (doc.score * 100).toFixed(1) : 'N/A'
      const relevanceIcon = doc.score >= 0.8 ? '🟢' : doc.score >= 0.6 ? '🟡' : '🔴'
      
      output += `### ${index + 1}. ${relevanceIcon} Document Match\n\n`
      
      // Metadata table
      output += `| Field | Value |\n`
      output += `|-------|-------|\n`
      output += `| **Relevance** | ${relevanceScore}% |\n`
      
      if (doc.metadata?.source) {
        output += `| **Source** | \`${doc.metadata.source}\` |\n`
      }
      
      if (doc.metadata?.document_type) {
        const typeIcon = doc.metadata.document_type === 'pdf' ? '📄' : '📝'
        output += `| **Type** | ${typeIcon} ${doc.metadata.document_type.toUpperCase()} |\n`
      }
      
      if (doc.metadata?.page_number !== undefined) {
        output += `| **Page** | ${doc.metadata.page_number} |\n`
      }
      
      if (doc.metadata?.chunk_index !== undefined && doc.metadata?.total_chunks !== undefined) {
        output += `| **Section** | ${doc.metadata.chunk_index + 1} of ${doc.metadata.total_chunks} |\n`
      }
      
      output += `\n`
      
      // Content preview
      if (doc.content) {
        const cleanContent = doc.content
          .replace(/\n+/g, ' ')  // Replace multiple newlines with single space
          .replace(/\s+/g, ' ')  // Replace multiple spaces with single space
          .replace(/`/g, '\\`')  // Escape backticks to prevent markdown conflicts
          .replace(/\*/g, '\\*')  // Escape asterisks
          .replace(/\#/g, '\\#')  // Escape hash symbols
          .replace(/\[/g, '\\[')  // Escape square brackets
          .replace(/\]/g, '\\]')  // Escape square brackets
          .trim()
        
        const maxLength = 300
        const truncatedContent = cleanContent.length > maxLength 
          ? cleanContent.substring(0, maxLength) + '...'
          : cleanContent
        
        output += `**📝 Content Preview:**\n\n`
        output += `\`\`\`\n${truncatedContent}\n\`\`\`\n\n`
      }
      
      // Separator between results
      if (index < response.results.length - 1) {
        output += `---\n\n`
      }
    })
    
    // Footer with usage tips
    output += `\n**💡 Pro Tips:**\n`
    output += `- Higher relevance scores (🟢) indicate better matches\n`
    output += `- Use specific technical terms for better results\n`
    output += `- Try \`/document health\` to check system status\n`
    
    return output
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
  
  // Format JSON responses nicely as fallback
  return '```json\n' + JSON.stringify(response, null, 2) + '\n```'
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
          content: `# 📋 Checkpoint Commands\n\n**Available Actions:**\n- \`/checkpoint list\` - List all checkpoints with details\n- \`/checkpoint get <thread_id>\` - Get detailed checkpoint information\n- \`/checkpoint exists <thread_id>\` - Check if checkpoint exists\n- \`/checkpoint delete <thread_id|checkpoint_id>\` - Delete specific checkpoint\n\n**Usage Notes:**\n- Use \`/checkpoint list\` first to see available checkpoints\n- Most commands work with either thread_id or checkpoint_id\n- Thread IDs look like: \`chat_1751781767673_jmtxv2lke\`\n- Checkpoint IDs look like: \`1f05a2ef-49d7-6d8e-8009-b2bc74329de3\``
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
            return { type: 'error', content: 'Please provide a session ID or checkpoint ID' }
          }
          try {
            const result = await executeAPICall(`/api/v1/checkpoints/${sessionId}`, 'DELETE')
            
            if (result.success === false) {
              return {
                type: 'warning',
                content: `⚠️ ${result.message}\n\n*Note: The delete command expects either a thread_id or checkpoint_id. Use '/checkpoint list' to see available checkpoints.*`
              }
            }
            
            return {
              type: 'success',
              content: formatResponse(result) || '✅ Checkpoint deleted successfully'
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
            // Document search uses query parameters, not JSON body
            const params = new URLSearchParams({
              query: query,
              limit: '5'
            })
            const result = await executeAPICall(`/api/v1/documents/search?${params}`, 'POST')
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

// Command sub-options
const COMMAND_SUB_OPTIONS: Record<string, string[]> = {
  memory: ['search', 'store', 'types', 'health'],
  checkpoint: ['get', 'exists', 'list', 'delete'],
  document: ['search', 'health'],
  alert: ['analyze'],
}

// Get command suggestions based on partial input
export function getCommandSuggestions(input: string): string[] {
  if (!input.startsWith('/')) return []
  
  const parts = input.slice(1).split(' ')
  const mainCommand = parts[0].toLowerCase()
  
  // If there's a space and we have sub-options, show sub-options
  if (parts.length === 2 && COMMAND_SUB_OPTIONS[mainCommand]) {
    const subPartial = parts[1].toLowerCase()
    return COMMAND_SUB_OPTIONS[mainCommand]
      .filter(sub => sub.startsWith(subPartial))
      .map(sub => `/${mainCommand} ${sub}`)
  }
  
  // If there are more than 2 parts, don't show suggestions (user is typing arguments)
  if (parts.length > 2) {
    return []
  }
  
  // Otherwise show main commands that match
  const suggestions: string[] = []
  for (const [name, cmd] of Object.entries(COMMANDS)) {
    if (name.toLowerCase().startsWith(mainCommand)) {
      suggestions.push(`/${name}`)
    }
  }
  
  return suggestions.sort()
}