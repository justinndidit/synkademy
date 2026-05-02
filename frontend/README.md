# Frontend — Synkademy

This folder contains the web client built with Vite + React + TypeScript.

## Requirements
- Node.js (LTS, e.g., 18+)
- npm, yarn or pnpm

## Setup
1. Install dependencies:

```bash
cd frontend
npm install
```

2. Create environment variables (optional): create a `.env` or `.env.local` and set values used by Vite. Common variables:

- `VITE_API_URL` — backend API base URL (e.g., `http://localhost:8080`)

## Development
Run the dev server with hot reload:

```bash
npm run dev
```

Open the app at `http://localhost:5173` (or the URL shown by Vite).

## Build
```bash
npm run build
npm run preview   # serve the built app locally
```

## Linting & Formatting
- `npm run lint` — run ESLint
- `npm run format` — run Prettier (if present)

## Notes
- Configure `VITE_API_URL` for connecting to the backend during development.
