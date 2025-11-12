# ===================== build stage =====================
FROM golang:1.25.4-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bot .

# ===================== runtime stage =====================
FROM gcr.io/distroless/base-debian12

# ==================== IMPORTANT ====================
# Перед запуском задайте TELEGRAM_BOT_TOKEN как переменную окружения.
# Пример: docker run -e TELEGRAM_BOT_TOKEN=123:ABC ...
# ================================================
ENV TELEGRAM_BOT_TOKEN=""

COPY --from=builder /bot /bot
USER nonroot:nonroot
ENTRYPOINT ["/bot"]
