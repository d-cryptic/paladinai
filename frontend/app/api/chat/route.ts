import { NextRequest, NextResponse } from 'next/server'

const PALADIN_API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8000'

export async function POST(req: NextRequest) {
  try {
    const body = await req.json()
    const { message, sessionId, additional_context } = body

    const response = await fetch(`${PALADIN_API_URL}/api/v1/chat`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        message,
        additional_context: {
          session_id: sessionId,
          ...additional_context,
        },
      }),
    })

    if (!response.ok) {
      const errorText = await response.text()
      console.error('Paladin API error:', response.status, errorText)
      throw new Error(`Paladin API error: ${errorText || response.statusText}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error: any) {
    console.error('Chat API error:', error)
    return NextResponse.json(
      { error: error.message || 'Failed to process chat message' },
      { status: 500 }
    )
  }
}