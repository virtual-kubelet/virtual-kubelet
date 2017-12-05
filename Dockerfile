FROM golang:alpine as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN	apk add --no-cache \
	ca-certificates

COPY . /go/src/github.com/virtual-kubelet/virtual-kubelet

RUN set -x \
	&& apk add --no-cache --virtual .build-deps \
		git \
		gcc \
		libc-dev \
		libgcc \
        make \
	&& cd /go/src/github.com/virtual-kubelet/virtual-kubelet \
	&& make build \ 
	&& apk del .build-deps \
    && cp bin/virtual-kubelet /usr/bin/virtual-kubelet \
	&& rm -rf /go \
	&& echo "Build complete."

FROM scratch

COPY --from=builder /usr/bin/virtual-kubelet /usr/bin/virtual-kubelet
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs

ENTRYPOINT [ "/usr/bin/virtual-kubelet" ]
CMD [ "--help" ]
