FROM docker.io/golang:1.18 AS build

RUN mkdir /build
WORKDIR /build

COPY main.go go.mod go.sum ./
COPY pkg/ ./pkg/

RUN go build -v -o bitcoind-exporter main.go

FROM registry.fedoraproject.org/fedora-minimal:36

COPY --from=build /build/bitcoind-exporter /usr/bin/
ENTRYPOINT [ "/usr/bin/bitcoind-exporter" ]
