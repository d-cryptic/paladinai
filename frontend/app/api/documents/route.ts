import { NextRequest, NextResponse } from 'next/server'

const PALADIN_API_URL = process.env.NEXT_PUBLIC_PALADIN_API_URL || 'http://localhost:8000'

export async function POST(req: NextRequest) {
  try {
    const formData = await req.formData()
    
    // Forward the FormData to Paladin API
    const response = await fetch(`${PALADIN_API_URL}/api/v1/documents/upload`, {
      method: 'POST',
      body: formData,
    })

    if (!response.ok) {
      const errorText = await response.text()
      throw new Error(`Paladin API error: ${errorText}`)
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('Document upload error:', error)
    return NextResponse.json(
      { error: 'Failed to upload document' },
      { status: 500 }
    )
  }
}