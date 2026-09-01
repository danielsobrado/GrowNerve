.PHONY: up down migrate gen test test-integration test-all run-api run-frontend run-browser reset firmware
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
test-integration:
	docker compose up -d postgres mosquitto
	GROWNERVE_TEST_DATABASE_URL="postgresql://postgres:postgres@127.0.0.1:5432/grownerve?sslmode=disable" \
	GROWNERVE_TEST_MQTT_BROKER="tcp://127.0.0.1:1883" \
	MIGRATION_DATABASE_URL="postgresql://postgres:postgres@127.0.0.1:5432/grownerve?sslmode=disable" \
	sh -c 'go run ./cmd/migrate up && go test -race ./...'
test-all:
	go test -race -cover ./...
	go vet ./...
	cd frontend && npm run typecheck && npm run lint && npm run test:coverage && npm run test:e2e && npm run build && npm run build:browser
run-api:
	go run ./cmd/api
run-frontend:
	cd frontend && npm run dev
run-browser:
	cd frontend && npm run dev:browser
reset:
	docker compose down --volumes
	docker compose up -d --build --wait
firmware:
	cd firmware/esp32 && pio run
