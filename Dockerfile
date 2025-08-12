FROM golang:1.24-alpine AS builder
WORKDIR /src


COPY go.mod go.sum ./
RUN go mod download


COPY . ./
RUN CGO_ENABLED=0 go build -o /out/backup .


FROM alpine:3.20
RUN apk add --no-cache \
    ca-certificates \
    mariadb-client \      
    postgresql16-client \  
    mongodb-tools \ 
    tzdata

WORKDIR /app
COPY --from=builder /out/backup /usr/local/bin/backup

ENTRYPOINT ["backup"]
