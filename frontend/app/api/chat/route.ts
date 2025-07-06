import { NextRequest, NextResponse } from 'next/server'

const PALADIN_API_URL = process.env.NEXT_PUBLIC_PALADIN_API_URL || 'http://localhost:8000'

export async function POST(req: NextRequest) {
  try {
    const body = await req.json()
    const { message, sessionId } = body

    const response = await fetch(`${PALADIN_API_URL}/api/v1/chat`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        message,
        additional_context: {
          session_id: sessionId,
        },
      }),
    })

    if (!response.ok) {
      throw new Error(`Paladin API error: ${response.statusText}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Chat API error:', error)
    return NextResponse.json(
      { error: 'Failed to process chat message' },
      { status: 500 }
    )
  }
}