# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o llm-box ./cmd/llm-box

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates bash && \
    adduser -D llmbox

COPY --from=builder /app/llm-box /usr/local/bin/llm-box
COPY --from=builder /app/install.sh /usr/local/share/llm-box/install.sh

ENV PATH="/usr/local/bin:${PATH}"

USER llmbox

ENTRYPOINT ["llm-box"]
CMD ["--help"]
