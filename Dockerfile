FROM golang:latest AS builder
ARG VERSION=0.0.0-unknown
ARG BRANCH=main

RUN groupadd -r fastly-exporter
RUN useradd -r -g fastly-exporter fastly-exporter

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

ADD .git .git
ADD cmd cmd
ADD pkg pkg

RUN env CGO_ENABLED=0 go build \
	-a \
	-ldflags="-X main.programVersion=$VERSION -X github.com/prometheus/common/version.Version=$VERSION -X github.com/prometheus/common/version.Branch=$BRANCH" \
	-o /fastly-historical-stats-exporter \
	./cmd/fastly-historical-stats-exporter

FROM scratch

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /fastly-historical-stats-exporter /fastly-historical-stats-exporter

USER fastly-exporter

EXPOSE 8080

ENTRYPOINT ["/fastly-historical-stats-exporter", "-listen=0.0.0.0:8080"]

LABEL org.opencontainers.image.title="fastly-historical-stats-exporter"
LABEL org.opencontainers.image.description="Prometheus exporter for Fastly Historical Stats API"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/saschanowak/fastly-historical-stats-exporter"
