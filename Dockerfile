FROM golang:1.22.5

WORKDIR /apps

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy all the source files
COPY . .

# Build the application
RUN go build -o myapp ./cmd

# Expose the required port
EXPOSE 8080

# Start the application
CMD ["./myapp"]