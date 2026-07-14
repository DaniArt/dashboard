FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY backend/ ./backend/
WORKDIR /build/backend
RUN go mod tidy && CGO_ENABLED=0 go build -o server ./cmd/server

FROM python:3.11-alpine
RUN apk --no-cache add ca-certificates
WORKDIR /app

# Install Python dependencies for VPN app
COPY vpn-app/requirements.txt /app/vpn-requirements.txt
RUN pip install --no-cache-dir -r /app/vpn-requirements.txt

# Copy Go binary
COPY --from=builder /build/backend/server /app/backend/server
COPY --from=builder /build/backend/config.yaml /app/backend/config.yaml
COPY --from=builder /build/backend/data /app/backend/data

# Copy VPN app
COPY vpn-app/ /app/vpn-app/

# Copy frontend (without AI)
COPY frontend/ /app/frontend

# Start script
RUN echo '#!/bin/sh' > /app/start.sh && \
    echo 'cd /app/vpn-app && python app.py &' >> /app/start.sh && \
    echo 'cd /app/backend && ./server' >> /app/start.sh && \
    chmod +x /app/start.sh

EXPOSE 8080
CMD ["/app/start.sh"]
