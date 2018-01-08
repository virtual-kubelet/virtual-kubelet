FROM debian:stretch-slim

WORKDIR /app
ADD ./rust-web-provider /app/rust-web-provider

ENV RUST_LOG=info
EXPOSE 3000

ENTRYPOINT [ "/app/rust-web-provider" ]
