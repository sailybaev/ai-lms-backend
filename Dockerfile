# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -o seed ./cmd/seed/main.go

# ── Stage 2: run ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/seed .

# Upload directory for avatars
RUN mkdir -p /app/public/uploads/avatars

EXPOSE 8080

ENV PORT=8080

CMD ["./server"]
