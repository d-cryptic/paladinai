/**
 * Memory-related functionality for Paladin AI
 */

import { executeAPICall } from './commands'

export interface Memory {
  memory: string
  score: number
  metadata: {
    memory_type?: string
    user_id?: string
    workflow_type?: string
    created_at?: string
    entities?: string[]
    [key: string]: any
  }
  related_entities?: any[]
}

export interface ContextualMemoryResponse {
  success: boolean
  memories: Memory[]
  context: string
  workflow_type: string
  error?: string
}

/**
 * Determine workflow type based on user input keywords
 */
export function determineWorkflowType(input: string): string {
  const lowerInput = input.toLowerCase()
  
  // INCIDENT workflow keywords
  if (['fix', 'resolve', 'troubleshoot', 'issue', 'problem', 'error'].some(keyword => 
    lowerInput.includes(keyword)
  )) {
    return 'INCIDENT'
  }
  
  // ACTION workflow keywords - include 'logs' as an action keyword
  if (['fetch', 'get', 'collect', 'action', 'restart', 'start', 'stop', 'execute', 'run', 'logs', 'show', 'display'].some(keyword => 
    lowerInput.includes(keyword)
  )) {
    return 'ACTION'
  }
  
  // Default to QUERY
  return 'QUERY'
}

/**
 * Fetch contextual memories based on user input
 */
export async function getContextualMemories(
  context: string,
  workflowType?: string,
  limit: number = 2
): Promise<ContextualMemoryResponse | null> {
  try {
    const params = new URLSearchParams({
      context,
      workflow_type: workflowType || determineWorkflowType(context),
      limit: limit.toString()
    })
    
    const response = await executeAPICall(`/api/memory/contextual?${params}`, 'GET')
    
    return response
  } catch (error) {
    console.error('Failed to fetch contextual memories:', error)
    return null
  }
}

/**
 * Format memories for display
 */
export function formatMemories(memories: Memory[]): string {
  if (!memories || memories.length === 0) {
    return ''
  }
  
  let formatted = '🧠 **Relevant Memories:**\n\n'
  
  memories.forEach((memory, index) => {
    const memoryType = memory.metadata?.memory_type || 'memory'
    const confidence = Math.round((memory.score || 0) * 100)
    
    formatted += `${index + 1}. **${memoryType}** (confidence: ${confidence}%) ${memory.memory || 'No content'}`
    
    if (index < memories.length - 1) {
      formatted += '\n'
    }
  })
  
  return formatted
}