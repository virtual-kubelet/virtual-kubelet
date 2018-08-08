# Building:
# docker build --no-cache -t vic-downstream -f Dockerfile.downstream .
# docker tag vic-downstream gcr.io/eminent-nation-87317/vic-downstream-trigger:1.x
# gcloud auth login
# gcloud docker -- push gcr.io/eminent-nation-87317/vic-downstream-trigger:1.x
# open vpn to CI cluster then run:
# docker tag vic-downstream wdc-harbor-ci.eng.vmware.com/default-project/vic-downstream-trigger:1.x
# docker push wdc-harbor-ci.eng.vmware.com/default-project/vic-downstream-trigger:1.x
FROM vmware/photon:2.0

RUN set -eux; \
     tdnf distro-sync --refresh -y; \
     tdnf install gzip -y; \
     tdnf install tar -y; \
     tdnf info installed; \
     tdnf clean all

RUN curl -L https://github.com/drone/drone-cli/releases/download/v0.8.5/drone_linux_amd64.tar.gz | tar zx && \
    install drone /usr/bin

RUN echo '#!/bin/bash' >> /usr/bin/trigger
RUN echo 'num=$(drone build ls --format "{{.Number}} {{.Status}}" --event push --branch "$DOWNSTREAM_BRANCH" "$DOWNSTREAM_REPO" | grep -v running | head -n1 | cut -d" " -f1)' >> /usr/bin/trigger
RUN echo 'for i in {1..5}; do drone build start "$DOWNSTREAM_REPO" $num && break || sleep 15; done' >> /usr/bin/trigger
RUN chmod +x /usr/bin/trigger

ENTRYPOINT ["trigger"]
