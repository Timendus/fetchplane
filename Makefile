IMAGE ?= timendus/fetchplane:latest

all: build docker-build

build:
	@GOOS=linux GOARCH=amd64 go build -o dist/linux/fetchplane-x86 ./cmd/fetchplane
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/linux/fetchplane-arm ./cmd/fetchplane
	@GOOS=windows GOARCH=amd64 go build -o dist/windows/fetchplane.exe ./cmd/fetchplane
	@GOOS=darwin GOARCH=amd64 go build -o dist/macos/fetchplane-intel ./cmd/fetchplane
	@GOOS=darwin GOARCH=arm64 go build -o dist/macos/fetchplane-apple-silicon ./cmd/fetchplane

docker-build:
	@docker build -t $(IMAGE) .
	@mkdir -p dist/docker
	@docker save $(IMAGE) | gzip > dist/docker/fetchplane.tar.gz

run:
	@STORAGE_PATH=storage go run ./cmd/fetchplane
