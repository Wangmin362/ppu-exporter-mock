# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.sum go.mod ./
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ppu-exporter main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/ppu-exporter .
EXPOSE 8080

CMD ["./ppu-exporter"]