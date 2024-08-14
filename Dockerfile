FROM golang:latest

WORKDIR /

# Download dependencies as a separate step to cache downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o main

# Run the binary
CMD ["./main"]
