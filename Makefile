cur_dir           := $(abspath $(shell git rev-parse --show-toplevel))
project_path      := /ceph-rebalancer
release_binary    := ceph-rebalancer
docker_container  := ceph-rebalancer

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
		-v $(cur_dir):/$(project_path) \
		-it $(container_build_version) \
		go test -v ./...
.PHONY: test

release:
	docker build \
		-t $(container_release_version) -f Dockerfile.release .
.PHONY: release

clean:
	rm -f $(release_binary)
.PHONY: clean
