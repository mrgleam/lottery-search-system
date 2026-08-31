.PHONY: help deps db db-down test race itest bench seed run psql clean load load-sweep scale-up stats

help:
	@echo "make deps    - download modules"
	@echo "make db      - start postgres in docker"
	@echo "make test    - unit tests (no database needed)"
	@echo "make race    - unit tests with the race detector"
	@echo "make itest   - integration tests (needs make db)"
	@echo "make bench   - benchmarks"
	@echo "make seed    - load 10,000,000 tickets"
	@echo "make run     - start the API on :8080"
	@echo "make load    - load test the running API"
	@echo "make load-sweep - sweep concurrency to find the knee"
	@echo "make stats   - database-side diagnostics"

deps:
	go mod tidy

db:
	docker compose up -d db
	@echo "waiting for postgres..."
	@until docker compose exec -T db pg_isready -U lottery -d lottery >/dev/null 2>&1; do sleep 1; done
	@echo "ready"

db-down:
	docker compose down

test:
	go test ./...

race:
	go test -race ./...

itest:
	go test -tags=integration -race ./postgres/...

bench:
	go test -bench=. -benchmem ./...

seed:
	go run ./cmd/seed -n 10000000

seed-small:
	go run ./cmd/seed -n 100000

run:
	go run ./cmd/api

psql:
	docker compose exec db psql -U lottery -d lottery

clean:
	docker compose down -v

load:
	go run ./cmd/loadgen -c 50 -d 30s -pattern mixed

load-sweep:
	@for c in 10 25 50 100 200 400; do \
		echo "=== concurrency $$c ==="; \
		go run ./cmd/loadgen -c $$c -d 15s -pattern mixed; \
	done

# Diagnostics for Part 1 of the scaling guide.
stats:
	docker compose exec -T db psql -U lottery -d lottery -c \
		"SELECT wait_event_type, wait_event, count(*) FROM pg_stat_activity WHERE state='active' GROUP BY 1,2;"
	docker compose exec -T db psql -U lottery -d lottery -c \
		"SELECT indexrelname, idx_scan, idx_tup_fetch FROM pg_stat_user_indexes WHERE relname='tickets';"
	docker compose exec -T db psql -U lottery -d lottery -c \
		"SELECT status, count(*) FROM tickets GROUP BY status ORDER BY status;"
