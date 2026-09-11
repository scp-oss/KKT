FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 gives a fully static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/kkt-monitor .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 kkt
WORKDIR /app
COPY --from=build /out/kkt-monitor /app/kkt-monitor

ENV LISTEN_ADDR=:8080 \
    DB_PATH=/data/kkt.db

VOLUME ["/data"]
RUN mkdir -p /data && chown kkt:kkt /data
USER kkt

EXPOSE 8080
ENTRYPOINT ["/app/kkt-monitor"]
