FROM golang:latest AS builder

WORKDIR /build

# Download dependencies as a separate step to cache downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN --mount=type=cache,target="/root/.cache/go-build" go test -v ./cmd/server/
RUN --mount=type=cache,target="/root/.cache/go-build" go build ./cmd/server

FROM debian
WORKDIR /app
COPY --from=builder /build/server /app/
CMD ["./server"]
