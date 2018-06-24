export DEBIAN_FRONTEND=noninteractive
export TEMP_DISK=/mnt

apt-get install -y -q --no-install-recommends \
            build-essential


# Add dockerce repo
apt-get update -y -q --no-install-recommends
apt-get install -y -q -o Dpkg::Options::="--force-confnew" --no-install-recommends \
    apt-transport-https ca-certificates curl software-properties-common cgroup-lite
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
apt-get update


#Install latest cuda driver..
CUDA_REPO_PKG=cuda-repo-ubuntu1604_9.1.85-1_amd64.deb
wget -O /tmp/${CUDA_REPO_PKG} http://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64/${CUDA_REPO_PKG} 
sudo dpkg -i /tmp/${CUDA_REPO_PKG}
sudo apt-key adv --fetch-keys http://developer.download.nvidia.com/compute/cuda/repos/ubuntu1604/x86_64/7fa2af80.pub 
rm -f /tmp/${CUDA_REPO_PKG}
sudo apt-get update -y -q --no-install-recommends
sudo apt-get install cuda-drivers -y -q --no-install-recommends

# install nvidia-docker
curl -fSsL https://nvidia.github.io/nvidia-docker/gpgkey | apt-key add -
curl -fSsL https://nvidia.github.io/nvidia-docker/ubuntu16.04/amd64/nvidia-docker.list | \
        tee /etc/apt/sources.list.d/nvidia-docker.list
apt-get update -y -q --no-install-recommends
apt-get install -y -q --no-install-recommends -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confnew" nvidia-docker2
systemctl restart docker.service
nvidia-docker version

# prep docker
systemctl stop docker.service
rm -rf /var/lib/docker
mkdir -p /etc/docker
mkdir -p $TEMPDISK/docker
chmod 777 $TEMPDISK/docker
echo "{ \"data-root\": \"$TEMP_DISK/docker\", \"hosts\": [ \"unix:///var/run/docker.sock\", \"tcp://127.0.0.1:2375\" ] }" > /etc/docker/daemon.json.merge
python -c "import json;a=json.load(open('/etc/docker/daemon.json.merge'));b=json.load(open('/etc/docker/daemon.json'));a.update(b);f=open('/etc/docker/daemon.json','w');json.dump(a,f);f.close();"
rm -f /etc/docker/daemon.json.merge
sed -i 's|^ExecStart=/usr/bin/dockerd.*|ExecStart=/usr/bin/dockerd|' /lib/systemd/system/docker.service
systemctl daemon-reload
systemctl start docker.service



