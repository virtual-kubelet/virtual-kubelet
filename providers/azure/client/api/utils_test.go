package api

import (
	"net/url"
	"testing"
)

const (
	baseURI = "https://management.azure.com"
)

type expandTest struct {
	in         string
	expansions map[string]string
	want       string
}

var expandTests = []expandTest{
	// no expansions
	{
		"",
		map[string]string{},
		"https://management.azure.com",
	},
	// multiple expansions, no escaping
	{
		"subscriptions/{{.subscriptionId}}/resourceGroups/{{.resourceGroup}}/providers/Microsoft.ContainerInstance/containerGroups/{{.containerGroupName}}",
		map[string]string{
			"subscriptionId":     "foo",
			"resourceGroup":      "bar",
			"containerGroupName": "baz",
		},
		"https://management.azure.com/subscriptions/foo/resourceGroups/bar/providers/Microsoft.ContainerInstance/containerGroups/baz",
	},
	// one expansion, with hex escapes
	{
		"subscriptions/{{.subscriptionId}}/resourceGroups/{{.resourceGroup}}/providers/Microsoft.ContainerInstance/containerGroups/{{.containerGroupName}}",
		map[string]string{
			"subscriptionId":     "foo/bar",
			"resourceGroup":      "bar",
			"containerGroupName": "baz",
		},
		"https://management.azure.com/subscriptions/foo%2Fbar/resourceGroups/bar/providers/Microsoft.ContainerInstance/containerGroups/baz",
	},
	// one expansion, with space
	{
		"subscriptions/{{.subscriptionId}}/resourceGroups/{{.resourceGroup}}/providers/Microsoft.ContainerInstance/containerGroups/{{.containerGroupName}}",
		map[string]string{
			"subscriptionId":     "foo and bar",
			"resourceGroup":      "bar",
			"containerGroupName": "baz",
		},
		"https://management.azure.com/subscriptions/foo%20and%20bar/resourceGroups/bar/providers/Microsoft.ContainerInstance/containerGroups/baz",
	},
	// expansion not found
	{
		"subscriptions/{{.subscriptionId}}/resourceGroups/{{.resourceGroup}}/providers/Microsoft.ContainerInstance/containerGroups/{{.containerGroupName}}",
		map[string]string{
			"subscriptionId":     "foo",
			"containerGroupName": "baz",
		},
		"https://management.azure.com/subscriptions/foo/resourceGroups/%3Cno%20value%3E/providers/Microsoft.ContainerInstance/containerGroups/baz",
	},
	// utf-8 characters
	{
		"{{.bucket}}/get",
		map[string]string{
			"bucket": "£100",
		},
		"https://management.azure.com/%C2%A3100/get",
	},
	// punctuations
	{
		"{{.bucket}}/get",
		map[string]string{
			"bucket": `/\@:,.`,
		},
		"https://management.azure.com/%2F%5C%40%3A%2C./get",
	},
	// mis-matched brackets
	{
		"/{{.bucket/get",
		map[string]string{
			"bucket": "red",
		},
		"https://management.azure.com/%7B%7B.bucket/get",
	},
}

func TestExpandURL(t *testing.T) {
	for i, test := range expandTests {
		uri := ResolveRelative(baseURI, test.in)
		u, err := url.Parse(uri)
		if err != nil {
			t.Fatalf("Parsing url %q failed: %v", test.in, err)
		}
		ExpandURL(u, test.expansions)
		got := u.String()
		if got != test.want {
			t.Errorf("got %q expected %q in test %d", got, test.want, i+1)
		}
	}
}
