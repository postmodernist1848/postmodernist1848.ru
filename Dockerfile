FROM golang:latest

WORKDIR /app

# Download dependencies as a separate step to cache downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Use cache on the host to speed up build times
RUN --mount=type=cache,target="/root/.cache/go-build" go build ./cmd/server

# Run the binary
CMD ["./server"]
