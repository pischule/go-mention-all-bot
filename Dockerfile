FROM golang:1.18-alpine as builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY . .
RUN go build -a -o bot

FROM alpine:3.16
COPY --from=builder /build/bot /app/bot
WORKDIR /app
RUN mkdir data
ENTRYPOINT ["./bot"]