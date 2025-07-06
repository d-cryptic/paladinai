# Paladin AI Frontend

A modern chat interface for the Paladin AI server built with Next.js, TypeScript, and Tailwind CSS.

## Features

- 💬 Real-time chat interface with Paladin AI
- 📁 Document upload for RAG processing
- 🔄 Session management with checkpoint integration
- 🌙 Dark theme by default
- 💾 Persistent chat history using Zustand
- 🎨 Minimal, clean design with shadcn/ui components

## Tech Stack

- **Framework**: Next.js 14 with App Router
- **Language**: TypeScript
- **Styling**: Tailwind CSS v3
- **UI Components**: shadcn/ui
- **State Management**: Zustand
- **HTTP Client**: Native fetch API
- **File Upload**: react-dropzone

## Getting Started

1. **Install dependencies**:
   ```bash
   npm install
   ```

2. **Set up environment variables**:
   ```bash
   cp .env.example .env.local
   ```
   Update `NEXT_PUBLIC_PALADIN_API_URL` if your Paladin server is running on a different URL.

3. **Start the development server**:
   ```bash
   npm run dev
   ```

4. **Open the app**:
   Navigate to [http://localhost:3000](http://localhost:3000)

## How It Works

### Session Management
The frontend leverages Paladin's checkpoint system for session persistence:
- Each chat creates a unique session ID (e.g., `chat_1234567890_abc123`)
- Messages are stored locally in Zustand for immediate UI updates
- Session IDs are passed to the Paladin server which automatically creates checkpoints
- Previous conversations are restored when using the same session ID

### Document Upload
- Drag and drop files or click the paperclip icon
- Files are sent to the `/api/v1/documents/rag` endpoint
- Documents are processed and chunked by the Paladin server
- System messages confirm successful uploads

### API Integration
The frontend communicates with the Paladin server through:
- `/api/v1/chat` - Send messages and receive AI responses
- `/api/v1/documents/rag` - Upload documents for processing
- `/api/v1/checkpoints` - Manage session checkpoints

## Project Structure

```
frontend/
├── app/
│   ├── api/           # Next.js API routes
│   ├── globals.css    # Global styles and Tailwind config
│   ├── layout.tsx     # Root layout with dark theme
│   └── page.tsx       # Main chat page
├── components/
│   ├── chat-input.tsx # Message input with file upload
│   ├── chat-message.tsx # Individual message display
│   ├── chat-view.tsx  # Main chat interface
│   └── sidebar.tsx    # Session list sidebar
├── lib/
│   ├── store.ts       # Zustand store for state management
│   └── utils.ts       # Utility functions
└── components.json    # shadcn/ui configuration
```

## Development

### Adding New Features
1. The Zustand store in `lib/store.ts` manages all chat state
2. API routes in `app/api/` proxy requests to the Paladin server
3. Components follow shadcn/ui patterns for consistency

### Styling
- Dark theme is enabled by default via the `dark` class on the html element
- Tailwind CSS classes are used throughout
- shadcn/ui provides pre-styled components

## Deployment

This is a standard Next.js application and can be deployed to:
- Vercel (recommended)
- Netlify
- Any Node.js hosting platform

Remember to set the `NEXT_PUBLIC_PALADIN_API_URL` environment variable in your deployment platform.
