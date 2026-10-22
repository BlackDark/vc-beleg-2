# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
# Multi-arch build without emulation: every build step runs on the build
# platform; only the final stage is assembled for the target platform.

FROM --platform=$BUILDPLATFORM node:24.22.0-alpine@sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
COPY web/tools/codegen/package.json tools/codegen/package.json
COPY web/project.inlang project.inlang
COPY web/messages messages
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS server
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/mahlzeit ./cmd/mahlzeit

FROM --platform=$BUILDPLATFORM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS typst
ARG TARGETARCH
ARG TYPST_VERSION=0.15.1
ARG TYPST_SHA256_AMD64=a6d077d0a95eed5a2eba715b2dae06be954f624ccbf85758a03f389ded33118c
ARG TYPST_SHA256_ARM64=5aa8d74a3d906e60ea12a66ac2f37f8eef1b14cbad7182a745e393a10c23dcee
RUN set -eu; \
    case "$TARGETARCH" in \
      amd64) arch=x86_64; sum=$TYPST_SHA256_AMD64 ;; \
      arm64) arch=aarch64; sum=$TYPST_SHA256_ARM64 ;; \
      *) echo "unsupported arch $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    wget -qO /tmp/typst.tar.xz "https://github.com/typst/typst/releases/download/v${TYPST_VERSION}/typst-${arch}-unknown-linux-musl.tar.xz"; \
    echo "$sum  /tmp/typst.tar.xz" | sha256sum -c -; \
    tar -xJf /tmp/typst.tar.xz -C /tmp; \
    install -m 0755 "/tmp/typst-${arch}-unknown-linux-musl/typst" /typst; \
    mkdir -p /rootfs/data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
# /data must exist and belong to nonroot, so a fresh named volume inherits
# a writable owner (distroless has no shell to fix it at runtime).
COPY --from=typst --chown=65532:65532 /rootfs/ /
COPY --from=typst /typst /usr/local/bin/typst
COPY --from=server /out/mahlzeit /usr/local/bin/mahlzeit
ENV MAHLZEIT_DATA_DIR=/data \
    MAHLZEIT_LISTEN=:8080 \
    MAHLZEIT_TYPST=/usr/local/bin/typst
VOLUME /data
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/usr/local/bin/mahlzeit", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/mahlzeit"]
CMD ["serve"]
