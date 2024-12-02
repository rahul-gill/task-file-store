# Stage 1
FROM golang:1.23.3 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux GOARCH=amd64 go build -o /app/client/store_client ./client/store_client.go
RUN GOOS=linux GOARCH=amd64 go build -o /app/server/store_server ./server/store_server.go

# Stage 2
FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/server/store_server .
EXPOSE 8080
RUN ls /app
CMD ["/app/store_server"]
