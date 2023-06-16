ARG GOLANG_CI_LINT_VERSION

FROM golang:1.20 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/virtual-kubelet/virtual-kubelet
WORKDIR /go/src/github.com/virtual-kubelet/virtual-kubelet
ARG BUILD_TAGS=""
RUN make VK_BUILD_TAGS="${BUILD_TAGS}" build
RUN cp bin/virtual-kubelet /usr/bin/virtual-kubelet

FROM golangci/golangci-lint:${GOLANG_CI_LINT_VERSION} as lint
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ARG OUT_FORMAT
RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/golangci-lint \
    golangci-lint run -v --out-format="${OUT_FORMAT:-colored-line-number}"

FROM scratch
COPY --from=builder /usr/bin/virtual-kubelet /usr/bin/virtual-kubelet
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs
ENTRYPOINT [ "/usr/bin/virtual-kubelet" ]
CMD [ "--help" ]
