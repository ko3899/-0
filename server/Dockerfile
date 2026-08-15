# 构建阶段
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /server .

# 运行阶段
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /server .
COPY migrations ./migrations
EXPOSE 8080
CMD ["./server"]
