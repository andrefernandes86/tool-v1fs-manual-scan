# Build stage - Go 1.24+ required by tm-v1-fs-golang-sdk (explicit tag avoids 1.21 cache)
FROM golang:1.24.4-alpine AS builder
WORKDIR /app

COPY go.mod ./
RUN go version && go mod download

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /v1fs-scanner .

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /v1fs-scanner .

ENV PORT=8080
ENV V1FS_CONFIG_PATH=/data/config.json
ENV V1FS_REPORTS_DIR=/data/reports

RUN mkdir -p /data/reports

# Malware Sample
##RUN mkdir -p /data/reports && \
##   mkdir -p /data/test-samples && \
##    echo 'X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*' \
##    > /data/test-samples/eicar.com

EXPOSE 8080

ENTRYPOINT ["/app/v1fs-scanner"]
