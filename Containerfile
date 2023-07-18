FROM docker.io/golang:1.20 AS build

ARG TARGETARCH
ARG GOARCH=${TARGETARCH}

RUN mkdir /build
WORKDIR /build

COPY main.go go.mod go.sum ./
COPY pkg/ ./pkg/

RUN go build -v -o bitcoind-exporter main.go

FROM registry.fedoraproject.org/fedora-minimal:38

COPY --from=build /build/bitcoind-exporter /usr/bin/
ENTRYPOINT [ "/usr/bin/bitcoind-exporter" ]
