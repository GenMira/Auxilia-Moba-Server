FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY *.go ./
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server .

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/server ./server
ENV PORT=8080 COOKIE_SECURE=true
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O - "http://127.0.0.1:${PORT}/healthz" >/dev/null || exit 1
ENTRYPOINT ["/app/server"]
