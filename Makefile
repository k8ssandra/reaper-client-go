.PHONY: start-services
start-services:
	docker-compose up -d

.PHONY: stop-services
stop-services:
	docker-compose down

.PHONY: wait-for-cluster-1
wait-for-cluster-1:
	./scripts/wait-for-cluster-ready.sh cluster-1-node-0

.PHONY: wait-for-cluster-2
wait-for-cluster-2:
	./scripts/wait-for-cluster-ready.sh cluster-2-node-0
