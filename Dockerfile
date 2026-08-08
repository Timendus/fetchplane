FROM golang:latest AS build

WORKDIR /src

COPY go.* ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/fetchplane ./cmd/fetchplane

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

ENV LIBRARY_PATH=/downloads
ENV CONFIG_PATH=/config

COPY --from=build /out/fetchplane /usr/local/bin/fetchplane

VOLUME ["/downloads", "/config"]
EXPOSE 38080

ENTRYPOINT ["/usr/local/bin/fetchplane"]
