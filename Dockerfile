FROM golang:alpine as builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -a -o bot

FROM alpine:latest

COPY --from=builder /app/bot /app/bot

WORKDIR /app

RUN mkdir data

ENTRYPOINT ["./bot"]