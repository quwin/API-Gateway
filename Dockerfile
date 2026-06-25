# Dockerfile

FROM golang:1.26.4-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o gateway ./cmd/gateway

FROM alpine:3.20

WORKDIR /app

COPY --from=build /app/gateway /app/gateway

EXPOSE 8080

CMD ["/app/gateway"]