# Build stage
FROM golang:1.24-bookworm AS builder

# Install libwebp for webp encoding support
RUN apt-get update && apt-get install -y --no-install-recommends \
    libwebp-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download || true

COPY . .
RUN go mod tidy && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o bot .

# Runtime stage
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app
COPY --from=builder /build/bot /app/bot

# Copy libwebp shared libraries from builder
COPY --from=builder /usr/lib/x86_64-linux-gnu/libwebp* /usr/lib/x86_64-linux-gnu/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libsharpyuv* /usr/lib/x86_64-linux-gnu/

EXPOSE 9090
EXPOSE 8080

ENTRYPOINT ["/app/bot"]
