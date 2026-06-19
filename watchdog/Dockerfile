FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /watchdog ./cmd/watchdog

FROM alpine:3.19

RUN apk add --no-cache ca-certificates inotify-tools

COPY --from=builder /watchdog /usr/local/bin/watchdog

ENTRYPOINT ["watchdog"]