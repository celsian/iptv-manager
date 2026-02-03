# Frontend build stage
FROM node:20-alpine AS frontend-builder

WORKDIR /app

# Copy package files
COPY web/package*.json ./web/
RUN cd web && npm ci

# Copy frontend source and build
COPY web/ ./web/
RUN cd web && npm run build

# Backend build stage
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend
COPY --from=frontend-builder /app/cmd/server/web/dist ./cmd/server/web/dist

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o iptv-manager ./cmd/server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=backend-builder /app/iptv-manager .

# Create data directory
RUN mkdir -p /data/playlists

# Set environment variables
ENV DATA_DIR=/data
ENV TZ=America/Los_Angeles

# Expose the web port
EXPOSE 8080

# Run the application
CMD ["./iptv-manager"]
