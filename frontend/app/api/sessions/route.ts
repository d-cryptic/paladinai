import { NextRequest, NextResponse } from 'next/server'

const PALADIN_API_URL = process.env.NEXT_PUBLIC_PALADIN_API_URL || 'http://localhost:8000'

export async function GET(req: NextRequest) {
  try {
    const { searchParams } = new URL(req.url)
    const sessionId = searchParams.get('sessionId')

    if (!sessionId) {
      // List all checkpoints
      const response = await fetch(`${PALADIN_API_URL}/api/v1/checkpoints/`)
      const data = await response.json()
      return NextResponse.json(data)
    }

    // Get specific checkpoint
    const response = await fetch(`${PALADIN_API_URL}/api/v1/checkpoints/${sessionId}`)
    
    if (!response.ok) {
      if (response.status === 404) {
        return NextResponse.json({ exists: false })
      }
      throw new Error(`Failed to fetch checkpoint: ${response.statusText}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Sessions API error:', error)
    return NextResponse.json(
      { error: 'Failed to fetch sessions' },
      { status: 500 }
    )
  }
}

export async function DELETE(req: NextRequest) {
  try {
    const { searchParams } = new URL(req.url)
    const sessionId = searchParams.get('sessionId')

    if (!sessionId) {
      return NextResponse.json(
        { error: 'Session ID is required' },
        { status: 400 }
      )
    }

    const response = await fetch(`${PALADIN_API_URL}/api/v1/checkpoints/${sessionId}`, {
      method: 'DELETE',
    })

    if (!response.ok) {
      throw new Error(`Failed to delete checkpoint: ${response.statusText}`)
    }

    return NextResponse.json({ success: true })
  } catch (error) {
    console.error('Delete session error:', error)
    return NextResponse.json(
      { error: 'Failed to delete session' },
      { status: 500 }
    )
  }
}