FROM golang:1.23-bookworm

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    chromium \
    ca-certificates \
    git \
    curl \
    wget \
    unzip \
    build-essential \
    pkg-config \
    fonts-liberation \
    libasound2 \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libcups2 \
    libdbus-1-3 \
    libdrm2 \
    libgbm1 \
    libglib2.0-0 \
    libgtk-3-0 \
    libnspr4 \
    libnss3 \
    libu2f-udev \
    libx11-6 \
    libx11-xcb1 \
    libxcb1 \
    libxcomposite1 \
    libxdamage1 \
    libxext6 \
    libxfixes3 \
    libxrandr2 \
    xdg-utils \
    && rm -rf /var/lib/apt/lists/*

RUN go install golang.org/x/tools/gopls@v0.18.0 
RUN go install github.com/go-delve/delve/cmd/dlv@v1.25.2

ENV CHROME_BIN=/usr/bin/chromium
ENV CHROMIUM_PATH=/usr/bin/chromium

WORKDIR /workspace

CMD ["/bin/bash"]
