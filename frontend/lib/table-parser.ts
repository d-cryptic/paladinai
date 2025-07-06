/**
 * Parse ASCII tables from text and ensure proper formatting
 */

export function parseAsciiTables(content: string): string {
  let processedContent = content;
  
  // Only fix double pipes that appear at the end of a cell and start of next row
  // This regex looks for | followed by space, then | at the start of what should be a new row
  processedContent = processedContent.replace(/\|\s+\|(?=\s*[a-zA-Z0-9\-])/g, '|\n|');
  
  // Also handle the case where header separator row is concatenated
  // Look for | followed by multiple dashes and another |
  processedContent = processedContent.replace(/\|\s+\|(?=[\s\-]+\|)/g, '|\n|');
  
  // Ensure tables have proper spacing before them if they come right after text
  processedContent = processedContent.replace(/([a-zA-Z0-9.!?])\s*(\|[^\n]+\|)/g, '$1\n\n$2');
  
  return processedContent;
}

/**
 * Check if content contains ASCII tables
 */
export function containsAsciiTable(content: string): boolean {
  return /\|[^\n]+\|/.test(content);
}