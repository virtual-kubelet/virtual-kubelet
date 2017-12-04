package urlutil

import "testing"

var (
	gitUrls = []string{
		"git://github.com/hyperhq/hypercli",
		"git@github.com:docker/docker.git",
		"git@bitbucket.org:atlassianlabs/atlassian-docker.git",
		"https://github.com/hyperhq/hypercli.git",
		"http://github.com/hyperhq/hypercli.git",
		"http://github.com/hyperhq/hypercli.git#branch",
		"http://github.com/hyperhq/hypercli.git#:dir",
	}
	incompleteGitUrls = []string{
		"github.com/hyperhq/hypercli",
	}
	invalidGitUrls = []string{
		"http://github.com/hyperhq/hypercli.git:#branch",
	}
)

func TestValidGitTransport(t *testing.T) {
	for _, url := range gitUrls {
		if IsGitTransport(url) == false {
			t.Fatalf("%q should be detected as valid Git prefix", url)
		}
	}

	for _, url := range incompleteGitUrls {
		if IsGitTransport(url) == true {
			t.Fatalf("%q should not be detected as valid Git prefix", url)
		}
	}
}

func TestIsGIT(t *testing.T) {
	for _, url := range gitUrls {
		if IsGitURL(url) == false {
			t.Fatalf("%q should be detected as valid Git url", url)
		}
	}

	for _, url := range incompleteGitUrls {
		if IsGitURL(url) == false {
			t.Fatalf("%q should be detected as valid Git url", url)
		}
	}

	for _, url := range invalidGitUrls {
		if IsGitURL(url) == true {
			t.Fatalf("%q should not be detected as valid Git prefix", url)
		}
	}
}
