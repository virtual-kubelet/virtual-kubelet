# Building:
# docker build --no-cache -t git-clone -f Dockerfile.gitclone .
# docker tag git-clone gcr.io/eminent-nation-87317/git-clone:1.x
# gcloud auth login
# gcloud docker -- push gcr.io/eminent-nation-87317/git-clone:1.x
# open vpn to CI cluster then run:
# docker tag git-clone 192.168.31.15/library/git-clone:1.x
# docker push 192.168.31.15/library/git-clone:1.x
FROM ubuntu

RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates

ADD git-clone.sh /usr/bin/git-clone.sh

ENTRYPOINT ["git-clone.sh"]
