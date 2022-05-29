FROM golang:1.18-alpine as builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY ["go.mod", "go.sum", "./"]
RUN go mod download

COPY *.go ./

RUN go build -a -o bot

FROM alpine:3.16

COPY --from=builder /app/bot /app/bot

WORKDIR /app

RUN mkdir data

ENTRYPOINT ["./bot"]