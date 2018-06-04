# Building:
# from vic root directory
# docker build --no-cache -t vic-machine-server -f cmd/vic-machine-server/Dockerfile .
# docker tag vic-machine-server gcr.io/eminent-nation-87317/vic-machine-server:1.x
# gcloud auth login
# gcloud docker -- push gcr.io/eminent-nation-87317/vic-machine-server:1.x

FROM vmware/photon:1.0

RUN set -eux; \
    tdnf distro-sync --refresh -y; \
    tdnf info installed; \
    tdnf clean all

ENV HOST 0.0.0.0
ENV PORT 80
ENV TLS_PORT 443

# Default location for TLS - Specify `-v /host/cert/path:/certs` to use defaults
# Override by providing a volume and values for `-e TLS_CERTIFICATE` and `-e TLS_PRIVATE_KEY`
ENV TLS_CERTIFICATE=/certs/server.crt
ENV TLS_PRIVATE_KEY=/certs/server.key

EXPOSE $PORT
EXPOSE $TLS_PORT

WORKDIR /opt/vmware/vsphere-integrated-containers/

COPY bin/vic-machine-server bin/
COPY bin/appliance.iso .
COPY bin/bootstrap.iso .

RUN setcap cap_net_bind_service=+ep bin/vic-machine-server

ENTRYPOINT bin/vic-machine-server
