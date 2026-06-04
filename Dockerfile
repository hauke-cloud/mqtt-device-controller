FROM golang:1.24-alpine AS builder
ARG GO_VERSION=1.24
ARG CGO_ENABLED=0

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o controller ./cmd/controller/

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/controller /controller
USER 65532:65532
ENTRYPOINT ["/controller"]
