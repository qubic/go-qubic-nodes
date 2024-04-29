FROM golang:1.22 AS builder

WORKDIR /src
COPY . /src

RUN go mod tidy
RUN go build -o "/src/bin/go-qubic-nodes"

# We don't need golang to run binaries, just use alpine.
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y ca-certificates
COPY --from=builder /src/bin/go-qubic-nodes /app/go-qubic-nodes
RUN chmod +x /app/go-qubic-nodes

EXPOSE 8080

WORKDIR /app

ENTRYPOINT ["./go-qubic-nodes"]
