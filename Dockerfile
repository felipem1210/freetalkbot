# Use a Go base image for the build stage
FROM golang:1.22-alpine AS builder

# Install gcc and other necessary tools for CGO
RUN apk update && \
    apk add --no-cache \
    build-base \
    soxr-dev \
    pkgconf
#git libtool m4 automake libpopt-dev

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./

# Add the whatsapp-media-decrypt tool
RUN go get github.com/ddz/whatsapp-media-decrypt && \
    go install github.com/ddz/whatsapp-media-decrypt

# Download the dependencies
RUN go mod tidy && go mod download 

# Copy the source code to the working directory
COPY . .

# Enable CGO and build the binary for the main application
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -tags sqlite_omit_load_extension -o /freetalkbot main.go

# Create a minimal image to run the binary
FROM alpine:latest

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite picotts soxr 

# Create a non-root user to run the application
RUN addgroup -g 1001 freetalkbot && \
    adduser --disabled-password \
    --no-create-home --uid 1001 --ingroup freetalkbot freetalkbot

# Set the working directory inside the container
WORKDIR /app/

# Create necessary files
RUN mkdir /app/audios

# Copy the binary from the build stage
COPY --from=builder /freetalkbot /usr/local/bin/freetalkbot
COPY --from=builder /go/bin/whatsapp-media-decrypt /usr/local/bin/whatsapp-media-decrypt

USER freetalkbot

# Expose the ports that the application will use
EXPOSE 8080 443 5034

# Default command to run the application
CMD ["freetalkbot"]
