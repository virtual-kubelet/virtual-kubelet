IMPORT_PATH := github.com/virtual-kubelet/virtual-kubelet
DOCKER_IMAGE := virtual-kubelet
exec := $(DOCKER_IMAGE)
github_repo := virtual-kubelet/virtual-kubelet
binary := virtual-kubelet
build_tags := "netgo osusergo $(VK_BUILD_TAGS)"

# comment this line out for quieter things
#V := 1 # When V is set, print commands and build progress.

# Space separated patterns of packages to skip in list, test, format.
IGNORED_PACKAGES := /vendor/

.PHONY: all
all: test build

.PHONY: safebuild
# safebuild builds inside a docker container with no clingons from your $GOPATH
safebuild:
	@echo "Building..."
	$Q docker build --build-arg BUILD_TAGS=$(build_tags) -t $(DOCKER_IMAGE):$(VERSION) .

.PHONY: build
build: authors
	@echo "Building..."
	$Q CGO_ENABLED=0 go build -a --tags $(build_tags) -ldflags '-extldflags "-static"' -o bin/$(binary) $(if $V,-v) $(VERSION_FLAGS) $(IMPORT_PATH)

.PHONY: tags
tags:
	@echo "Listing tags..."
	$Q @git tag

.PHONY: release
release: build $(GOPATH)/bin/goreleaser
	goreleaser


### Code not in the repository root? Another binary? Add to the path like this.
# .PHONY: otherbin
# otherbin: .GOPATH/.ok
#   $Q go install $(if $V,-v) $(VERSION_FLAGS) $(IMPORT_PATH)/cmd/otherbin

##### ^^^^^^ EDIT ABOVE ^^^^^^ #####

##### =====> Utility targets <===== #####

.PHONY: clean test list cover format docker deps

deps: setup
	@echo "Ensuring Dependencies..."
	$Q go env
	$Q dep ensure

docker:
	@echo "Docker Build..."
	$Q docker build --build-arg BUILD_TAGS="$(VK_BUILD_TAGS)" -t $(DOCKER_IMAGE) .

clean:
	@echo "Clean..."
	$Q rm -rf bin


test:
	@echo "Testing..."
	$Q go test $(if $V,-v) -i $(allpackages) # install -race libs to speed up next run
ifndef CI
	@echo "Testing Outside CI..."
	$Q go vet $(allpackages)
	$Q GODEBUG=cgocheck=2 go test $(allpackages)
else
	@echo "Testing in CI..."
	$Q mkdir -p test
	$Q ( go vet $(allpackages); echo $$? ) | \
       tee test/vet.txt | sed '$$ d'; exit $$(tail -1 test/vet.txt)
	$Q ( GODEBUG=cgocheck=2 go test -v $(allpackages); echo $$? ) | \
       tee test/output.txt | sed '$$ d'; exit $$(tail -1 test/output.txt)
endif

list:
	@echo "List..."
	@echo $(allpackages)

cover: $(GOPATH)/bin/gocovmerge
	@echo "Coverage Report..."
	@echo "NOTE: make cover does not exit 1 on failure, don't use it to check for tests success!"
	$Q rm -f .GOPATH/cover/*.out cover/all.merged
	$(if $V,@echo "-- go test -coverpkg=./... -coverprofile=cover/... ./...")
	@for MOD in $(allpackages); do \
        go test -coverpkg=`echo $(allpackages)|tr " " ","` \
            -coverprofile=cover/unit-`echo $$MOD|tr "/" "_"`.out \
            $$MOD 2>&1 | grep -v "no packages being tested depend on"; \
    done
	$Q gocovmerge cover/*.out > cover/all.merged
ifndef CI
	@echo "Coverage Report..."
	$Q go tool cover -html .GOPATH/cover/all.merged
else
	@echo "Coverage Report In CI..."
	$Q go tool cover -html .GOPATH/cover/all.merged -o .GOPATH/cover/all.html
endif
	@echo ""
	@echo "=====> Total test coverage: <====="
	@echo ""
	$Q go tool cover -func .GOPATH/cover/all.merged

format: $(GOPATH)/bin/goimports
	@echo "Formatting..."
	$Q find . -iname \*.go | grep -v \
        -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)) | xargs goimports -w

# skaffold.run deploys the virtual-kubelet to the Kubernetes cluster targeted by the current kubeconfig using skaffold.
.PHONY: skaffold.run
skaffold.run: MODE ?= dev
skaffold.run: PROFILE ?= local
skaffold.run:
	@GOOS=linux GOARCH=amd64 $(MAKE) build
	@skaffold $(MODE) \
		-f $(PWD)/hack/skaffold/virtual-kubelet/skaffold.yml \
		-p $(PROFILE)

##### =====> Internals <===== #####

.PHONY: setup
setup: clean
	@echo "Setup..."
	if ! grep "/bin" .gitignore > /dev/null 2>&1; then \
        echo "/bin" >> .gitignore; \
    fi
	if ! grep "/cover" .gitignore > /dev/null 2>&1; then \
        echo "/cover" >> .gitignore; \
    fi
	if ! grep "/bin" .gitignore > /dev/null 2>&1; then \
        echo "/bin" >> .gitignore; \
    fi
	if ! grep "/test" .gitignore > /dev/null 2>&1; then \
        echo "/test" >> .gitignore; \
    fi
	mkdir -p cover
	mkdir -p bin
	mkdir -p test
	go get -u github.com/golang/dep/cmd/dep
	go get github.com/wadey/gocovmerge
	go get golang.org/x/tools/cmd/goimports
	go get github.com/mitchellh/gox
	go get github.com/goreleaser/goreleaser

VERSION          := $(shell git describe --tags --always --dirty="-dev")
DATE             := $(shell date -u '+%Y-%m-%d-%H:%M UTC')
VERSION_FLAGS    := -ldflags='-X "github.com/virtual-kubelet/virtual-kubelet/version.Version=$(VERSION)" -X "github.com/virtual-kubelet/virtual-kubelet/version.BuildTime=$(DATE)"'

# assuming go 1.9 here!!
_allpackages = $(shell go list ./...)

# memoize allpackages, so that it's executed only once and only if used
allpackages = $(if $(__allpackages),,$(eval __allpackages := $$(_allpackages)))$(__allpackages)


Q := $(if $V,,@)


$(GOPATH)/bin/gocovmerge:
	@echo "Checking Coverage Tool Installation..."
	@test -d $(GOPATH)/src/github.com/wadey/gocovmerge || \
        { echo "Vendored gocovmerge not found, try running 'make setup'..."; exit 1; }
	$Q go install github.com/wadey/gocovmerge
$(GOPATH)/bin/goimports:
	@echo "Checking Import Tool Installation..."
	@test -d $(GOPATH)/src/golang.org/x/tools/cmd/goimports || \
        { echo "Vendored goimports not found, try running 'make setup'..."; exit 1; }
	$Q go install golang.org/x/tools/cmd/goimports

$(GOPATH)/bin/goreleaser:
	go get -u github.com/goreleaser/goreleaser

authors:
	$Q git log --all --format='%aN <%cE>' | sort -u  | sed -n '/github/!p' > GITAUTHORS
	$Q cat AUTHORS GITAUTHORS  | sort -u > NEWAUTHORS
	$Q mv NEWAUTHORS AUTHORS
	$Q rm -f NEWAUTHORS
	$Q rm -f GITAUTHORS
