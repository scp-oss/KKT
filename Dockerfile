FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 gives a fully static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/kkt-monitor .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata su-exec && \
    adduser -D -u 10001 kkt
WORKDIR /app
COPY --from=build /out/kkt-monitor /app/kkt-monitor
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

# PORT can be overridden at `docker run`/compose time (-e PORT=9090) to move
# the service off 8080; just remember to publish the same port with -p.
ENV PORT=8080 \
    DB_PATH=/data/kkt.db

VOLUME ["/data"]
RUN mkdir -p /data && chown kkt:kkt /data

# Stays root here on purpose - the entrypoint chowns /data (see
# docker-entrypoint.sh for why) and then drops to the unprivileged "kkt"
# user itself before the app binary ever runs.
EXPOSE 8080
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/kkt-monitor"]
