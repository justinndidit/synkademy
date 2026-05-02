# Backend — Synkademy

This folder contains the Go-based HTTP API for Synkademy.

## Requirements
- Go 1.20+ (or the version specified in `go.mod`)
- Make (optional)
- Docker (optional)

## Environment
The backend expects configuration via environment variables. Common variables:

- `PORT` — HTTP port (default: `8080`)
- `DATABASE_URL` — PostgreSQL connection string (optional for production)
- `JWT_SECRET` — secret used for signing JWTs
- `REDIS_URL` — Redis URL for signalling or presence (optional)

Create a `.env` file or export vars in your shell for local development.

## Run (local)
From the repository root or `backend` directory:

```bash
cd backend
go run ./cmd/server
```

Or build a binary:

```bash
go build -o synkademy ./cmd/server
./synkademy
```

## Docker
Build a Docker image for the backend:

```bash
docker build -t synkademy-backend ./backend
```

You can also use the project's `docker-compose.yml` for an integrated development environment.

## Testing
Use `go test ./...` in the `backend` directory to run unit tests (if present).

## Contributing
- Keep APIs stable where possible; add migrations when changing DB models.
