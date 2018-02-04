# Building step
FROM golang:1.9.3
COPY . /go/src/github.com/nats-io/nats-kubernetes/operators/nats-server
WORKDIR /go/src/github.com/nats-io/nats-kubernetes/operators/nats-server
RUN CGO_ENABLED=0 go build -o nats-server-operator -v -a cmd/operator/main.go

# Single binary image
FROM scratch
COPY --from=0 /go/src/github.com/nats-io/nats-kubernetes/operators/nats-server/nats-server-operator /nats-server-operator
ENTRYPOINT ["/nats-server-operator"]
