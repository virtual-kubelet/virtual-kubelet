package nuczzz

import "testing"

// go test -mod=vendor -run="^TestParseConfig$" -v
func TestParseConfig(t *testing.T) {
	var config = &Config{}
	if err := parseConfig("example.yaml", config); err != nil {
		t.Fatal(err.Error())
	}

	t.Logf("config: %#v", config)
}
