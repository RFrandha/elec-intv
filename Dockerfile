FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src/ ./src/
COPY migrations/ ./migrations/

RUN CGO_ENABLED=0 GOOS=linux go build -o /pricing-engine ./src/cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /pricing-engine .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./pricing-engine"]
