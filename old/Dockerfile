FROM golang:latest AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN --mount=type=cache,target="/root/.cache/go-build" go build -o server .

FROM debian
WORKDIR /app
COPY --from=builder /build/assets/ /app/assets
COPY --from=builder /build/static/ /app/static
COPY --from=builder /build/contents/ /app/contents
COPY --from=builder /build/server /app/
CMD ["./server"]
