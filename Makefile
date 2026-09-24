.PHONY: test build up down ingest
test:
	docker run --rm -v "$(PWD):/src" -w /src golang:1.23-alpine sh -c 'go mod tidy && go test ./...'
build:
	docker build --build-arg TARGET=api -t insights-api:local .
	docker build --build-arg TARGET=worker -t insights-worker:local .
up:
	docker compose up --build -d
down:
	docker compose down
ingest:
	curl -fsS -X POST http://localhost:8080/v1/sandbox/ingest -H 'Content-Type: application/json' --data-binary @mockdata/sample.json
