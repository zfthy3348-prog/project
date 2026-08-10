# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# نسخ ملفات المشروع
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o download-page .

# === المرحلة النهائية ===
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# نسخ الملف الثنائي وملف القالب
COPY --from=builder /app/download-page .
COPY --from=builder /app/index.html .

EXPOSE 8080

CMD ["./download-page"]
