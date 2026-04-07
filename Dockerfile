FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY server/ .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o chat-server .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/chat-server /chat-server
EXPOSE 8080
CMD ["/chat-server"]
