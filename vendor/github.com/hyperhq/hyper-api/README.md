# Go client for the Hyper.sh API

The `hyper` command uses this package to communicate with the Hyper.sh server. It can also be used by your own Go applications to do anything the command-line interface does – running containers, pulling images, managing volumes, etc.

For example, to list running containers (the equivalent of `hyper ps`):

```go
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/hyperhq/hyper-api/client"
	"github.com/hyperhq/hyper-api/types"
)

func main() {
	var (
		host          = "tcp://us-west-1.hyper.sh:443"
		customHeaders = map[string]string{}
		verStr        = "v1.23"
		accessKey     = "xx"
		secretKey     = "xxx"
	)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	client, err := client.NewClient(host, verStr, httpClient, customHeaders, accessKey, secretKey)
	if err != nil {
		panic(err)
	}
	containers, err := client.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		fmt.Printf("%s\t%s\n", container.ID[:10], container.Image)
	}
}
```

Full documentation is available on [document](https://docs.hyper.sh).


## License

engine-api is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.
