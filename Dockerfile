FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/fetchplane ./cmd/fetchplane

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

ENV STORAGE_PATH=/data

COPY --from=build /out/fetchplane /usr/local/bin/fetchplane

VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/fetchplane"]
CMD ["-daemon"]
