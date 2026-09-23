FROM golang:1.25-alpine AS builder

WORKDIR /app

# Only the module definition is copied first so Docker can cache dependency setup.
COPY go.mod ./
RUN go mod download

COPY . .

# Make sure the folder exists even if no images were added yet.
RUN mkdir -p templates/app_icon_url

# Build a static binary without a shell dependency at runtime.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build     -trimpath     -ldflags="-s -w"     -o /out/download-page .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata     && addgroup -S app     && adduser -S -G app -h /app app

WORKDIR /app

COPY --from=builder /out/download-page /app/download-page
COPY --from=builder /app/index.html /app/index.html
COPY --from=builder /app/templates /app/templates

RUN chown -R app:app /app

USER app

ENV PORT=8080

EXPOSE 8080

ENTRYPOINT ["/app/download-page"]
