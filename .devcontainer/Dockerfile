FROM golang:1.23-bookworm

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    git curl unzip build-essential pkg-config
    && rm -rf /var/lib/apt/lists/*

RUN go install golang.org/x/tools/gopls@v0.18.0 
RUN go install github.com/go-delve/delve/cmd/dlv@v1.25.2

WORKDIR /workspace

CMD ["/bin/bash"]
