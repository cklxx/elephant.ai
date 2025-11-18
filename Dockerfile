FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o alex ./cmd/alex

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/alex /usr/local/bin/alex

USER 65532:65532
WORKDIR /workspace

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/alex", "--help"]

ENTRYPOINT ["/usr/local/bin/alex"]
CMD ["--help"]
