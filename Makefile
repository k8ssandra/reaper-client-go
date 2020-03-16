.PHONY: start-services
start-services:
	docker-compose up -d

.PHONY: stop-services
stop-services:
	docker-compose down

.PHONY: wait-for-cluster-1
wait-for-cluster-1:
	./scripts/wait-for-cluster-ready.sh cluster-1-node-0 2

.PHONY: wait-for-cluster-2
wait-for-cluster-2:
	./scripts/wait-for-cluster-ready.sh cluster-2-node-0 2

.PHONY: wait-for-cluster-3
wait-for-cluster-3:
	./scripts/wait-for-cluster-ready.sh cluster-3-node-0 1

.PHONY: wait-for-reaper
wait-for-reaper:
	./scripts/wait-for-reaper-ready.sh

.PHONY: setup-tests
setup-tests: wait-for-reaper wait-for-cluster-1 wait-for-cluster-2 wait-for-cluster-3
	curl -H "Content-Type: application/json" -d '{"seedHost": "cluster-1}' --request POST http://localhost:8080/cluster
	curl -H "Content-Type: application/json" -d '{"seedHost": "cluster-2}' --request POST http://localhost:8080/cluster

.PHONY: test
test: setup-tests
	@echo Running tests:
	go test -v -race -cover ./reaper/...

.PHONY: test-nowait
test-nowait:
	@echo Running tests:
	go test -v -race -cover ./reaper/...