# Stage 1: builder
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o nanobot ./cmd/nanobot

# Stage 2: runtime
FROM gcr.io/distroless/static-debian12

COPY --from=builder /build/nanobot /nanobot

ENTRYPOINT ["/nanobot"]
