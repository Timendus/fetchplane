IMAGE ?= fetchplane:latest

.PHONY: build docker-build run

build:
	go build ./...

docker-build:
	docker build -t $(IMAGE) .

run:
	docker run --rm -it --env-file .env -v $(PWD)/downloads:/data $(IMAGE)
