FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /build/metrics-agent ./cmd/agent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /build/metrics-agent /app/metrics-agent

ENV HOST_PROC=/host/proc
ENV HOST_SYS=/host/sys
ENV HOST_ETC=/host/etc
ENTRYPOINT ["/app/metrics-agent"]
