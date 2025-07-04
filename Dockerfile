FROM golang:1.24.4-bookworm as builder

RUN addgroup --system nonroot && adduser --system --ingroup nonroot --uid 65532 nonroot
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=linux \
    go build \
    -buildvcs=false \
    -trimpath \
    -ldflags="-s -w" \
    -o api

FROM scratch
COPY --from=builder --chmod=0111 /app/api /api
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
USER 65532
ENTRYPOINT ["/api"]
