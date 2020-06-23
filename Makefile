cur_dir           := $(abspath $(shell git rev-parse --show-toplevel))
project_path      := /archimedes
release_binary    := archimedes
docker_container  := archimedes

container_build_version   := $(docker_container):build
container_release_version := $(docker_container):latest

build:
	docker build \
		-t $(container_build_version) -f Dockerfile.build .
.PHONY: build

test: build
	docker run --rm \
		-e GOPRIVATE=github.com/digitalocean \
		-w $(project_path) \
		-v $(cur_dir):$(project_path) \
		$(container_build_version) \
		go test -v ./...
.PHONY: test

release:
	docker build \
		-t $(container_release_version) -f Dockerfile.release .
.PHONY: release

clean:
	rm -f $(release_binary)
.PHONY: clean
