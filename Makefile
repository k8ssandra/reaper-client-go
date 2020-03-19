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

.PHONY: test
test:
	@echo Running tests:
	go test -v -race -cover ./reaper/...

.PHONY: test-cleanup
test-cleanup:
	@echo Running test-cleanup
	rm -rf data/cassandra
	rm -rf data/reaper

.PHONY: publish-release
publish-release:
	ghr -t ${GITHUB_TOKEN} -u ${CIRCLE_PROJECT_USERNAME} -r ${CIRCLE_PROJECT_REPONAME} -c ${CIRCLE_SHA1} -debug -replace ${CIRCLE_TAG}