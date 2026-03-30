BIN_NAME=monitor-api
VERSION?=0.0.0
IMAGE_NAME?=api-health-monitor
IMAGE_TAG?=dev
IMAGE?=${IMAGE_NAME}:${IMAGE_TAG}
CONTAINER_NAME?=$(BIN_NAME)
K8S_NAMESPACE?=pulse-check
K8S_DEPLOYMENT?=$(BIN_NAME)
K8S_IMAGE?=$(BIN_NAME):$(IMAGE_TAG)
K8S_TAR?=$(BIN_NAME).tar

CYAN := $(shell tput -Txterm setaf 6)
WHITE := $(shell tput -Txterm setaf 7)
GREEN := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET := $(shell tput -Txterm sgr0)

.PHONY: all test run build clean dep tidy docker-build docker-run docker-run-detached docker-release docker-stop k3s-build k3s-deploy k3s-rollout k8s-apply help

all: help

#build
build: ## Build your project and put the output in bin/
	mkdir -p bin
# 	GOARCH=amd64 GOOS=darwin go build -o bin/${BIN_NAME}-darwin ${MAIN}
	GOARCH=amd64 GOOS=linux go build -o bin/${BIN_NAME}-linux ./cmd/${BIN_NAME}
# 	GOARCH=amd64 GOOS=windows go build -o bin/${BIN_NAME}-windows ${MAIN}

run:	build ##Run project
	./bin/${BIN_NAME}-linux

clean: ## Remove build related file
	go clean
	rm -fr ./bin

dep: ## Download dependecies
	go mod download

tidy: ## Tidy up the go.mod file
	go mod tidy

#test
test: ## Run tests of the project
	go test -v -race ./...

test_coverage: ## Run tests and export the coverage
	@mkdir -p test
	go test ./... -cover -covermode=count -coverprofile=test/coverage.out
	go tool cover -func=test/coverage.out

#docker
docker-build:
	docker build --rm -t ${IMAGE} .

docker-run:
	mkdir -p data
	touch data/monitor.db
	docker run --rm \
		--name ${CONTAINER_NAME} \
		-p 8080:8080 \
		-e ADDR=:8080 \
		-e DB_PATH=/data/monitor.db \
		-v $(PWD)/data:/data \
		${IMAGE}

docker-run-detached:
	mkdir -p data
	touch data/monitor.db
	docker run -d \
		--name ${CONTAINER_NAME} \
		-p 8080:8080 \
		-e ADDR=:8080 \
		-e DB_PATH=/data/monitor.db \
		-v $(PWD)/data:/data \
		${IMAGE}

docker-release:
	docker tag ${IMAGE} ${IMAGE_NAME}:latest
	docker tag ${IMAGE} ${IMAGE_NAME}:${VERSION}
	docker push ${IMAGE_NAME}:latest
	docker push ${IMAGE_NAME}:${VERSION}

docker-stop:
	@docker stop ${CONTAINER_NAME} >/dev/null 2>&1 || true
	@docker rm ${CONTAINER_NAME} >/dev/null 2>&1 || true
	@docker ps -q --filter ancestor=${IMAGE} | xargs -r docker stop >/dev/null 2>&1 || true
	@docker ps -aq --filter ancestor=${IMAGE} | xargs -r docker rm >/dev/null 2>&1 || true

#k3s
k3s-build: ## Build image, export tar and import into k3s containerd
	docker build --rm -t ${K8S_IMAGE} .
	docker save ${K8S_IMAGE} -o ${K8S_TAR}
	sudo k3s ctr images import ${K8S_TAR}
	rm -f ${K8S_TAR}

k3s-deploy: k3s-build k8s-apply k3s-rollout ## Full deploy: build, apply manifests, rollout restart

k8s-apply: ## Apply all Kubernetes manifests
	sudo k3s kubectl apply -f deployments/k8s/

k3s-rollout: ## Restart the k3s deployment to pick up the new image
	sudo k3s kubectl rollout restart deployment/${K8S_DEPLOYMENT} -n ${K8S_NAMESPACE}
	sudo k3s kubectl rollout status deployment/${K8S_DEPLOYMENT} -n ${K8S_NAMESPACE}

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
