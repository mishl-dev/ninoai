FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o ninoai ./main.go

FROM alpine:3.19
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 ninoai
WORKDIR /app
COPY --from=builder /build/ninoai .
USER ninoai
CMD ["./ninoai"]