FROM --platform=linux/amd64 debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl wget bzip2 \
    pkg-config libssl-dev libudev-dev \
    gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*

ENV SOLANA_VERSION=2.3.13
ENV PLATFORM_TOOLS_VERSION=v1.54

RUN curl -sSfL --http1.1 --retry 3 --retry-delay 5 \
    "https://release.anza.xyz/v${SOLANA_VERSION}/solana-release-x86_64-unknown-linux-gnu.tar.bz2" \
    -o /tmp/solana.tar.bz2 && \
    tar xjf /tmp/solana.tar.bz2 -C /tmp && \
    mkdir -p /opt/solana && \
    cp -r /tmp/solana-release/* /opt/solana/ && \
    rm -rf /tmp/solana-release /tmp/solana.tar.bz2

RUN mkdir -p /opt/platform-tools && \
    wget --tries=3 --waitretry=5 -q \
    "https://github.com/anza-xyz/platform-tools/releases/download/${PLATFORM_TOOLS_VERSION}/platform-tools-linux-x86_64.tar.bz2" \
    -O /tmp/pt.tar.bz2 && \
    tar xjf /tmp/pt.tar.bz2 -C /opt/platform-tools && \
    rm /tmp/pt.tar.bz2

ENV PATH="/opt/platform-tools/llvm/bin:/opt/platform-tools/rust/bin:/opt/solana/bin:${PATH}"
ENV RUSTC="/opt/platform-tools/rust/bin/rustc"
ENV CARGO_HOME="/opt/cargo"

COPY program/ /build/

RUN cd /build/programs/solock && \
    cargo-build-sbf --no-rustup-override --tools-version ${PLATFORM_TOOLS_VERSION} 2>&1 || true && \
    echo "Dependency cache warmed"

COPY docker/build.sh /usr/local/bin/build-solock
RUN chmod +x /usr/local/bin/build-solock

WORKDIR /build
ENTRYPOINT ["build-solock"]
