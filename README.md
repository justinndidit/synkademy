# Synkademy

Synkademy is a lightweight full-stack video conferencing platform focused on running classes for educational institutions and hosting webinars. It provides role-aware meeting controls, recordings, chat, and an extensible backend to integrate with institutional systems.

## Features
- Real-time video conferencing for classes and webinars
- Role-based controls (host/instructor, attendee/student)
- Chat, mute/unmute, screen sharing (frontend-ready)
- Deployable via Docker Compose or individually (frontend/backend)

## Architecture
- Frontend: Vite + React + TypeScript (folder: `frontend`)
- Backend: Go HTTP API (folder: `backend`)
- Optional: PostgreSQL / Redis for persistence & signalling (configurable)

## Quickstart (development)
Prerequisites: Docker & Docker Compose (or Node and Go for local development).

1. Start everything with Docker Compose (recommended for dev):

```bash
docker-compose up --build
```

2. Frontend will be available at `http://localhost:5173` (or as configured).
3. Backend API will listen on the port configured in `backend` (see `backend/README.md`).

## Repositories
- Frontend: `frontend` — see `frontend/README.md` for dev setup.
- Backend: `backend` — see `backend/README.md` for dev setup.

## Contributing
Contributions, bug reports and feature requests are welcome. Please open issues and PRs.

## License
This project is provided as-is. Add a LICENSE file to set the project's license.
