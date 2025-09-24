# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application
RUN make build

# Final stage
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    openssh-client \
    curl \
    bash \
    python3 \
    py3-pip \
    && rm -rf /var/cache/apk/*

# Create ansible user
RUN addgroup -g 1000 ansible && \
    adduser -u 1000 -G ansible -D -h /home/ansible ansible

# Create necessary directories
RUN mkdir -p /etc/ansible \
    /var/log/ansible \
    /var/cache/ansible \
    /usr/share/ansible \
    && chown -R ansible:ansible /etc/ansible /var/log/ansible /var/cache/ansible

# Copy the binary from builder stage
COPY --from=builder /app/build/ansible /usr/local/bin/ansible

# Copy default configuration
COPY --chown=ansible:ansible configs/ansible.yaml /etc/ansible/

# Set proper permissions
RUN chmod +x /usr/local/bin/ansible

# Switch to ansible user
USER ansible
WORKDIR /home/ansible

# Create SSH directory
RUN mkdir -p ~/.ssh && chmod 700 ~/.ssh

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ansible --version || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/ansible"]
CMD ["--help"]

# Labels
LABEL org.opencontainers.image.title="Ansible Go" \
      org.opencontainers.image.description="Ansible automation platform written in Go" \
      org.opencontainers.image.source="https://github.com/work-obs/ansible-go" \
      org.opencontainers.image.version="2.19.0-go" \
      org.opencontainers.image.licenses="Apache-2.0"