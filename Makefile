.PHONY: up down migrate gen test test-all run-api run-frontend run-browser reset
up:
	docker compose up -d --build --wait
down:
	docker compose down
migrate:
	go run ./cmd/migrate up
gen:
	sqlc generate
	oapi-codegen --config api/oapi-codegen.yaml api/openapi.yaml
	cd frontend && npm run gen:api
test:
	go test ./...
	cd frontend && npm test
test-all:
	go test -race -cover ./...
	go vet ./...
	cd frontend && npm run typecheck && npm run lint && npm run test:coverage && npm run build && npm run build:browser
run-api:
	go run ./cmd/api
run-frontend:
	cd frontend && npm run dev
run-browser:
	cd frontend && npm run dev:browser
reset:
	docker compose down --volumes
	docker compose up -d --build --wait
