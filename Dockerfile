# Builder Stage
FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/minibot ./cmd/appname

# Final minimalistic Stage
FROM alpine:latest

# Install basic CA certificates and curl since it might do web searches
RUN apk --no-cache add ca-certificates curl bash

# Install the binary
COPY --from=builder /bin/minibot /usr/local/bin/minibot

# Setup entrypoint dir
WORKDIR /app
ENTRYPOINT ["minibot"]
CMD ["gateway"]
