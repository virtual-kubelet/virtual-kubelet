FROM golang:1.12 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/virtual-kubelet/virtual-kubelet
WORKDIR /go/src/github.com/virtual-kubelet/virtual-kubelet
ARG BUILD_TAGS=""
RUN make VK_BUILD_TAGS="${BUILD_TAGS}" build
RUN cp bin/virtual-kubelet /usr/bin/virtual-kubelet

FROM scratch
COPY --from=builder /usr/bin/virtual-kubelet /usr/bin/virtual-kubelet
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs
ENTRYPOINT [ "/usr/bin/virtual-kubelet" ]
CMD [ "--help" ]
