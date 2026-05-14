FROM golang:alpine AS builder
RUN apk add --no-cache gcc musl-dev libwebp-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-linkmode external -extldflags '-static'" \
    -o server .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
