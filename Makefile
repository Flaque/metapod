PKGS := $(shell go list ./... | grep -v vendor)

build:
	go build

.PHONY: run
run: build
	@metapod

.PHONY: test
test:
	go test $(PKGS)