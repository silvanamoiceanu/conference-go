FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o conference-go ./cmd/server/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/conference-go .
COPY --from=builder /app/web ./web

EXPOSE 8080

CMD ["./conference-go", "-web"]
