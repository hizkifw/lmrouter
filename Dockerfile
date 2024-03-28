FROM golang:1.22-alpine AS base
FROM base AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-extldflags '-static' -s -w" \
    -tags netgo \
    -o lmrouter

FROM base as ca-certificates
RUN apk add -U --no-cache ca-certificates

FROM scratch
USER 1000
WORKDIR /app

COPY --from=ca-certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /src/lmrouter /app/lmrouter

CMD ["/app/lmrouter"]