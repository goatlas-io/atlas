# Stage 1 - Go, Build the Binary
FROM golang:1.16.6 as go-builder
WORKDIR /src/atlas
ENV GO111MODULE=on
COPY . /src/atlas
ARG branch=master
ENV BRANCH=${branch}
RUN make release && cp atlas /go/bin/atlas

# Stage 2 - Download Binaries
FROM appropriate/curl as binaries
ENV TINI_VERSION v0.18.0
RUN curl --fail -sLo /tini https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-static-amd64

# Stage X - Final Image
FROM debian:stretch-slim
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/bin/atlas"]

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
RUN useradd -r -u 999 -d /home/atlas atlas

COPY --from=binaries /tini /usr/bin/tini
COPY --from=go-builder /go/bin/atlas /usr/bin/atlas
RUN chmod +x /usr/bin/tini /usr/bin/atlas

USER atlas
