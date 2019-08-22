$pwd = (Get-Location).Path

docker run --security-opt seccomp:unconfined -it `
	-v ${pwd}:/gopath/src/github.com/virtual-kubelet/virtual-kubelet `
	-w /gopath/src/github.com/virtual-kubelet/virtual-kubelet `
		golang /bin/bash
