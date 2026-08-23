# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o aflare ./cmd/aflare

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates bash && \
    adduser -D aflare

COPY --from=builder /app/aflare /usr/local/bin/aflare
COPY --from=builder /app/install.sh /usr/local/share/aflare/install.sh

ENV PATH="/usr/local/bin:${PATH}"

USER aflare

ENTRYPOINT ["aflare"]
CMD ["--help"]
