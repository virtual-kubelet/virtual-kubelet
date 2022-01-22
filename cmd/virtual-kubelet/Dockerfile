FROM alpine:latest

COPY vk /
RUN chmod +x /vk

RUN mkdir /cert
COPY cert/server.crt /cert
COPY cert/server.key /cert

COPY example.yaml /

COPY config /config

ENTRYPOINT ["/vk"]
