FROM golang:1.24-alpine AS builder
WORKDIR /src


COPY go.mod go.sum ./
RUN go mod download


COPY . ./
RUN CGO_ENABLED=0 go build -o /out/backup .

# ---------- runtime ----------
FROM alpine:latest

RUN apk add --no-cache \
    ca-certificates \
    mysql-client \
    mariadb-connector-c \
    postgresql17-client \  
    mongodb-tools \ 
    tzdata

WORKDIR /app
COPY --from=builder /out/backup /usr/local/bin/backup

ENTRYPOINT ["backup"]
