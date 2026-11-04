ARG GO_VERSION=1.26.4
FROM golang:${GO_VERSION}-alpine AS builder
ARG SERVICE_NAME=admin
ARG APP_VERSION=1.0.0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    service_package="./${SERVICE_NAME}/cmd/server"; \
    if [ "${SERVICE_NAME}" = "admin" ]; then service_package="./admin"; fi; \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${APP_VERSION}" \
    -o /out/server "${service_package}"

FROM alpine:3.22
RUN apk --no-cache add ca-certificates tzdata && adduser -D appuser
ARG WITH_PRINT_BROWSER=0
RUN if [ "$WITH_PRINT_BROWSER" = "1" ]; then apk --no-cache add chromium font-noto-cjk; fi
WORKDIR /app
COPY --from=builder /out/server /app/bin/server
RUN mkdir -p /app/data /tmp/printrender && chown -R appuser:appuser /app /tmp/printrender
USER appuser:appuser
ENTRYPOINT ["/app/bin/server"]
