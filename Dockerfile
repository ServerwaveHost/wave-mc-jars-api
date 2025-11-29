# Build stage
FROM golang:1.25 AS builder

WORKDIR /app

COPY . .
RUN go mod download

# Build
RUN CGO_ENABLED=0 go build -o main .

# Final stage
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/main .
COPY --from=builder /app/java.json .

# Expose port
EXPOSE 8080

# Run
CMD ["./main"]
