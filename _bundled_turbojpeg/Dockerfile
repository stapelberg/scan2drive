FROM debian:bookworm

RUN dpkg --add-architecture arm64

RUN apt-get update && apt-get install -y libturbojpeg0-dev:arm64 libjpeg62-turbo-dev:arm64
