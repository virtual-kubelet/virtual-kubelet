FROM golang:alpine as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN apk add --no-cache \
	ca-certificates \
	--virtual .build-deps \
	git \
	gcc \
	libc-dev \
	libgcc \
	make \
	bash

COPY . /go/src/github.com/virtual-kubelet/virtual-kubelet
WORKDIR /go/src/github.com/virtual-kubelet/virtual-kubelet
ARG BUILD_TAGS="netgo osusergo"
RUN make VK_BUILD_TAGS="${BUILD_TAGS}" build
RUN cp bin/virtual-kubelet /usr/bin/virtual-kubelet


FROM scratch
COPY --from=builder /usr/bin/virtual-kubelet /usr/bin/virtual-kubelet
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs
ENTRYPOINT [ "/usr/bin/virtual-kubelet" ]
CMD [ "--help" ]
