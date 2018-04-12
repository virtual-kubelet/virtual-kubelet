FROM debian:stretch-slim

WORKDIR /app
ADD ./web-rust /app/web-rust

ENV RUST_LOG=info
EXPOSE 3000

ENTRYPOINT [ "/app/web-rust" ]
