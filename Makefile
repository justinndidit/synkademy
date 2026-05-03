BACKEND_PATH = ./backend
FRONTEND_PATH=./frontend
MIGRATIONS_PATH = ./backend/migrations

.PHONY:

dev-ui:
	cd $(FRONTEND_PATH) && npm run dev

dev-server:
	cd $(BACKEND_PATH) && air


docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

help:
	@echo ""
	@echo "  make dev-ui               Run the frontend in dev mode"
	@echo "  make dev server         	 Run the backend in dev mode"
	@echo ""
	@echo ""
	@echo "  make docker-up            Start containers"
	@echo "  make docker-down          Stop containers"
	@echo "  make docker-logs          Tail container logs"
	@echo ""