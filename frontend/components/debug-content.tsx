'use client'

export function DebugContent({ content }: { content: string }) {
  // Show the raw content for debugging
  return (
    <div className="mt-4 p-4 bg-muted rounded-md">
      <h4 className="text-sm font-semibold mb-2">Debug: Raw Content</h4>
      <pre className="text-xs whitespace-pre-wrap overflow-auto">
        {content}
      </pre>
      <h4 className="text-sm font-semibold mt-4 mb-2">Debug: Escaped</h4>
      <pre className="text-xs whitespace-pre-wrap overflow-auto">
        {JSON.stringify(content)}
      </pre>
    </div>
  )
}