FROM golang:1.23-alpine AS builder

# set working directory
WORKDIR /app

# copy the go mod and sum files
COPY go.mod go.sum ./

# download dependencies
RUN go mod download

# copy the rest of the application
COPY . .

# build the application
RUN go build -o main

FROM alpine:latest

# Set non-root user for security (optional but recommended)
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# copy the binary from the builder stage
COPY --from=builder /app/main /app/main

WORKDIR /app

# export port 1985
EXPOSE 1985

# Switch to non-root user
USER appuser

# Run the application
CMD ["./main"]
