OS = $(shell uname | tr A-Z a-z)

# Dependency versions
GOTESTSUM_VERSION = 0.6.0

.PHONY: build test run docker run-backends clean all lint fmt vet tidy coverage

all: clean build run

clean:
	@rm -f ./proxy

lint: bin/golangci-lint tidy fmt vet
	@./bin/golangci-lint run
	
vet:
	@go vet ./...

fmt:
	@go fmt ./...
	
build:
	@mkdir -p bin
	@go build -o proxy

tidy:
	@go mod tidy

test: bin/gotestsum-${GOTESTSUM_VERSION}
	@./bin/gotestsum-${GOTESTSUM_VERSION} --no-summary=skipped --junitfile ./coverage.xml --format short-verbose -- -coverprofile=./coverage.txt -covermode=atomic ./...

run: build
	./proxy --config ./sample_config.yaml --kubeconfig $$HOME/.kube/config

coverage:
	go tool cover -func coverage.txt

docker:
	@docker build --platform=linux/amd64 -t patnaikshekhar/workspace-proxy:1.1 -f ./deploy/Dockerfile .

bin/gotestsum-${GOTESTSUM_VERSION}:
	@mkdir -p bin
	@curl -L https://github.com/gotestyourself/gotestsum/releases/download/v${GOTESTSUM_VERSION}/gotestsum_${GOTESTSUM_VERSION}_${OS}_amd64.tar.gz | tar -zOxf - gotestsum > ./bin/gotestsum-${GOTESTSUM_VERSION} && chmod +x ./bin/gotestsum-${GOTESTSUM_VERSION}

bin/golangci-lint:
	@mkdir -p bin
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s 

run-backends:
	docker rm -vf nginx && \
	docker run -d --name nginx -p 8090:80 nginx && \
	docker rm -vf ttyd && \
	docker run -d --name ttyd -p 8091:7681 tsl0922/ttyd && \
	docker rm -vf vscode && \
	docker run -d --name vscode -p 8092:3000 gitpod/openvscode-server
