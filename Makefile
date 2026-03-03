BIN_NAME=monitor-api
MAIN=cmd/monitor-api/main.go
VERSION?=0.0.0
DOCKER_REGISTRY?= #if set, it should finish with '/'

CYAN := $(shell tput -Txterm setaf 6)
WHITE := $(shell tput -Txterm setaf 7)
GREEN := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET := $(shell tput -Txterm sgr0)

.PHONY: all test run

all: help

#build
build: ## Build your project and put the output in bin/
	mkdir -p bin
# 	GOARCH=amd64 GOOS=darwin go build -o bin/${BIN_NAME}-darwin ${MAIN}
	GOARCH=amd64 GOOS=linux go build -o bin/${BIN_NAME}-linux ${MAIN}
# 	GOARCH=amd64 GOOS=windows go build -o bin/${BIN_NAME}-windows ${MAIN}

run:	build ##Run project
	./bin/${BIN_NAME}-linux

clean: ## Remove build related file
	go clean
	rm -fr ./bin

dep: ## Download dependecies
	go mod download

#test
test: ## Run tests of the project
	go test -v -race ./...

test_coverage: ## Run tests and export the coverage
	@mkdir -p test
	go test ./... -cover -covermode=count -coverprofile=test/coverage.out
	go tool cover -func=test/coverage.out

#docker
docker-build:
	docker build --rm --tag ${BIN_NAME} .

docker-release:
	docker tag ${BIN_NAME} ${DOCKER_REGISTRY}${BIN_NAME}:latest
	docker tag ${BIN_NAME} ${DOCKER_REGISTRY}${BIN_NAME}:${VERSION}
	docker push ${DOCKER_REGISTRY}${BIN_NAME}:latest
	docker push ${DOCKER_REGISTRY}${BIN_NAME}:${VERSION}

help:
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
