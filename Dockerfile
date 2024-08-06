# Use a Go base image for the build stage
FROM golang:1.22-bookworm AS builder

# Install gcc and other necessary tools for CGO
RUN apt-get update && apt-get install -y gcc git libtool m4 automake libpopt-dev

# Set the working directory inside the container
WORKDIR /app

# Download and install picotts
RUN git clone https://github.com/ihuguet/picotts.git && \
    cd picotts/pico && \
    ./autogen.sh && \
    ./configure && \
    make && \
    make install

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
FROM debian:bookworm-slim

ARG SQL_DB_FILE_NAME

# Install necessary runtime dependencies
RUN apt-get update && apt-get install -y ca-certificates tzdata sqlite3 libpopt0
# Optionally, you might want to clean up the package lists to reduce image size
RUN apt-get clean && rm -rf /var/lib/apt/lists/*

# Create a non-root user to run the application
RUN useradd -u 1001 freetalkbot
USER freetalkbot

# Set the working directory inside the container
WORKDIR /app/

RUN touch /app/${SQL_DB_FILE_NAME} && \
    mkdir /app/audios

# Copy the binary from the build stage
COPY --from=builder /freetalkbot /usr/local/bin/freetalkbot
COPY --from=builder /go/bin/whatsapp-media-decrypt /usr/local/bin/whatsapp-media-decrypt
# Copy the shared libraries, binary and languages needed for picotts
COPY --from=builder /usr/local/bin/pico2wave /usr/local/bin/pico2wave
COPY --from=builder /usr/local/lib/libttspico*.so* /usr/local/lib/
COPY --from=builder /usr/local/share/pico/lang /usr/local/share/pico/lang

# Set the library path
ENV LD_LIBRARY_PATH=/usr/local/lib

# Expose the ports that the application will use
EXPOSE 8080 443 5034

# Default command to run the application
CMD ["freetalkbot"]
