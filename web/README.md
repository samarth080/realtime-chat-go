# realtime-chat-go — Frontend

React + TypeScript frontend for the P2P chat app. See the [root README](../README.md) for full project documentation.

## Stack

- React 18 + TypeScript
- Vite
- Tailwind CSS
- Zustand (with persist middleware)

## Dev

```bash
npm install
npm run dev
```

Set `web/.env.local`:
```
VITE_WS_URL=ws://localhost:8080
VITE_API_URL=http://localhost:8080
```

## Build

```bash
npm run build
# output in dist/
```
