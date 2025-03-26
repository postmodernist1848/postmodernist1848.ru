FROM golang:alpine AS builder

WORKDIR /build

# Download dependencies as a separate step to cache downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Use cache on the host to speed up build times
RUN --mount=type=cache,target="/root/.cache/go-build" go build ./cmd/server

FROM golang:alpine
WORKDIR /app
COPY --from=builder /build/server /app/
CMD ["./server"]

