# Use a Go base image for the build stage
FROM golang:1.22-bookworm AS builder

# Install gcc and other necessary tools for CGO
RUN apt-get update && apt-get install -y gcc

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./

RUN go get github.com/ddz/whatsapp-media-decrypt && \
    go install github.com/ddz/whatsapp-media-decrypt

RUN which whatsapp-media-decrypt

RUN go mod tidy && go mod download 

# Copy the source code to the working directory
COPY . .

# Enable CGO and build the binary for the main application
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -tags sqlite_omit_load_extension -o /freetalkbot main.go

# Create a minimal image to run the binary
FROM debian:bookworm-slim

ARG SQL_DB_FILE_NAME

# Install necessary runtime dependencies
RUN apt-get update && apt-get install -y ca-certificates tzdata sqlite3


# Create a non-root user to run the application
#RUN useradd -u 1001 freetalkbot
#USER freetalkbot

# Set the working directory inside the container
WORKDIR /app/

RUN touch /app/${SQL_DB_FILE_NAME} && \
    mkdir /app/audios

# Copy the binary from the build stage
COPY --from=builder /freetalkbot .
COPY --from=builder /go/bin/whatsapp-media-decrypt /usr/local/bin/whatsapp-media-decrypt

# Expose the ports that the application will use
EXPOSE 8080 443 5034

# Default command to run the application
CMD ["./freetalkbot"]
