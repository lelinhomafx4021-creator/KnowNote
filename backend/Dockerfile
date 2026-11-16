FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates tzdata libreoffice-writer libreoffice-impress \
        libreoffice-calc fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 knownote \
    && useradd --uid 10001 --gid knownote --create-home --home-dir /home/knownote knownote \
    && mkdir -p /app/data \
    && chown -R knownote:knownote /app
WORKDIR /app
ENV DATA_DIR=/app/data
# LibreOffice 需要可写的用户目录来保存无界面运行配置。
ENV HOME=/home/knownote
COPY --from=build /out/server /app/server
USER knownote:knownote
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/server"]
