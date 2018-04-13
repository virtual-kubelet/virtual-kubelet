FROM golang:1.10.1-alpine

RUN apk update && apk add git docker musl-dev gcc curl bash jq

# Download and install the latest release of dep
RUN curl https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 -L -o /usr/bin/dep
RUN chmod +x /usr/bin/dep
RUN go get -u github.com/alecthomas/gometalinter
RUN gometalinter --install


# Restore dep for dispatcher
WORKDIR /go/src/github.com/lawrencegripper/pod2docker
COPY ./Gopkg.lock .
COPY ./Gopkg.toml .
RUN dep ensure -vendor-only
COPY . .
RUN gometalinter --vendor --disable-all --exclude=/go/src/ --enable=errcheck --enable=vet --enable=gofmt --enable=golint --enable=deadcode --enable=varcheck --enable=structcheck --deadline=5m ./...
ENTRYPOINT [ "go" ]
CMD ["test", "./..."] 
