# syntax=docker/dockerfile:1

# ── Builder ────────────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Layer cache: dependencies change less often than source code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/sriganeshlokesh/forged/config.Version=${VERSION}" \
    -o /out/forged \
    ./cmd

# ── Final image ────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/forged /forged

# EXPOSE is documentation only — Railway routes to the value of $PORT at runtime.
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/forged"]
