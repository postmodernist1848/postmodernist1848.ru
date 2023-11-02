# Use an official Golang runtime as a parent image
FROM golang:latest

WORKDIR /

# Copy the current directory contents into the container at /app
COPY . .

# Build the Go app
RUN go build -o main .

# Expose port 8080 for incoming traffic
EXPOSE 8080

# Define the command to run the app when the container starts
CMD ["/main"]
