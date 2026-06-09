IMAGE ?= fetchplane:latest

.PHONY: build docker-build run

build:
	go build ./...

docker-build:
	docker build -t $(IMAGE) .

run:
	docker run --rm -it -v $(PWD)/data:/data $(IMAGE)
