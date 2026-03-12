# Stage 1: Build the Next.js UI
FROM node:20-alpine AS ui-builder
WORKDIR /app/shachiku-ui

# Install pnpm and dependencies
RUN npm install -g pnpm
COPY shachiku-ui/package.json shachiku-ui/pnpm-lock.yaml* ./
RUN pnpm install

# Copy source code and build
COPY shachiku-ui/ ./
RUN pnpm run build

# Stage 2: Build the Go Backend
FROM golang:1.25.1-bookworm AS go-builder
WORKDIR /app/shachiku

# Download Go module dependencies
COPY shachiku/go.mod shachiku/go.sum ./
RUN go mod download

# Copy backend source code
COPY shachiku/ ./

# Copy built UI static files into the expected location for go:embed
RUN mkdir -p ui/dist
COPY --from=ui-builder /app/shachiku-ui/out/ ./ui/dist/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/shachiku-bin main.go

# Stage 3: Final Production Image
FROM ubuntu:24.04

# Playwright dependencies & CA certificates
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    curl \
    wget \
    gnupg \
    libnss3 \
    libnspr4 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    libdrm2 \
    libxkbcommon0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libasound2t64 \
    libpango-1.0-0 \
    libcairo2 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the compiled executable from Go builder
COPY --from=go-builder /app/shachiku-bin /app/shachiku
RUN chmod +x /app/shachiku

# Expose default port
EXPOSE 8080

# Configure Environment Variables
ENV GIN_MODE=release
# IS_PUBLIC=true ensures the API binds to 0.0.0.0 instead of 127.0.0.1
ENV IS_PUBLIC=true

CMD ["/app/shachiku"]
