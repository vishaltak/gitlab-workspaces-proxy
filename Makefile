.PHONY: build
build:
	go build -o proxy

.PHONY: test
test:
	go test -v ./...

.PHONY: run
run: build
	./proxy --config ./sample_config.yaml

.PHONY: run-backends
run-backends:
	docker rm -vf nginx && \
	docker run -d --name nginx -p 8090:80 nginx && \
	docker rm -vf ttyd && \
	docker run -d --name ttyd -p 8091:7681 tsl0922/ttyd && \
	docker rm -vf vscode && \
	docker run -d --name vscode -p 8092:3000 gitpod/openvscode-server
