# Builder stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
# sqlite requires CGO, so we need gcc/musl-dev
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with CGO enabled for sqlite
RUN CGO_ENABLED=1 go build -o llmreq main.go

# Runner stage
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache sqlite-libs

COPY --from=builder /app/llmreq .
# Copy config if needed, or rely on env/volume
# COPY litellm_config.yaml .

EXPOSE 8080

CMD ["./llmreq"]
