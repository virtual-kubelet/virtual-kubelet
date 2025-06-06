SHELL := /bin/bash

DOCKER_IMAGE := virtual-kubelet
exec := $(DOCKER_IMAGE)
github_repo := virtual-kubelet/virtual-kubelet
binary := virtual-kubelet

GOTEST ?= go test $(if $V,-v)

export GO111MODULE ?= on

include Makefile.e2e
# Currently this looks for a globally installed gobin. When we move to modules,
# should consider installing it locally
# Also, we will want to lock our tool versions using go mod:
# https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
gobin_tool ?= $(shell which gobin || echo $(GOPATH)/bin/gobin)
goimports := golang.org/x/tools/cmd/goimports@release-branch.go1.15
gocovmerge := github.com/wadey/gocovmerge@b5bfa59ec0adc420475f97f89b58045c721d761c
goreleaser := github.com/goreleaser/goreleaser@v0.82.2
gox := github.com/mitchellh/gox@v1.0.1

# comment this line out for quieter things
# V := 1 # When V is set, try to enable extra logging for debugging

# Space separated patterns of packages to skip in list, test, format.
IGNORED_PACKAGES := /vendor/

TEST_OS := $(shell go env GOOS)
TEST_ARCH := $(shell go env GOARCH)

.PHONY: all
all: test build

.PHONY: safebuild
# safebuild builds inside a docker container with no clingons from your $GOPATH
safebuild:
	@echo "Building..."
	docker build --build-arg BUILD_TAGS="$(VK_BUILD_TAGS)" -t $(DOCKER_IMAGE):$(VERSION) .

.PHONY: build
build: build_tags := netgo osusergo
build: OUTPUT_DIR ?= bin
build: authors
	@echo "Building..."
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -o $(OUTPUT_DIR)/$(binary) $(if $V,-v) $(VERSION_FLAGS) ./cmd/$(binary)

.PHONY: tags
tags:
	@echo "Listing tags..."
	@git tag

.PHONY: release
release: build goreleaser
	$(gobin_tool) -run $(goreleaser)

##### =====> Utility targets <===== #####

.PHONY: clean test list cover format docker
mod:
	@echo "Prune Dependencies..."
	go mod tidy

docker:
	@echo "Docker Build..."
	docker build --build-arg BUILD_TAGS="$(VK_BUILD_TAGS)" -t $(DOCKER_IMAGE) .

clean:
	@echo "Clean..."
	rm -rf bin

vet:
	@echo "go vet'ing..."
ifndef CI
	@echo "go vet'ing Outside CI..."
	go vet $(TESTDIRS)
else
	@echo "go vet'ing in CI..."
	mkdir -p test
	( go vet $(TESTDIRS); echo $$? ) | \
       tee test/vet.txt | sed '$$ d'; exit $$(tail -1 test/vet.txt)
endif

test:
	$(GOTEST) $(TESTDIRS)

list:
	@echo "List..."
	@echo $(TESTDIRS)

cover: gocovmerge
	@echo "Coverage Report..."
	@echo "NOTE: make cover does not exit 1 on failure, don't use it to check for tests success!"
	rm -f .GOPATH/cover/*.out cover/all.merged
	$(if $V,@echo "-- go test -coverpkg=./... -coverprofile=cover/... ./...")
	@for MOD in $(TESTDIRS); do \
        go test -coverpkg=`echo $(TESTDIRS)|tr " " ","` \
            -coverprofile=cover/unit-`echo $$MOD|tr "/" "_"`.out \
            $$MOD 2>&1 | grep -v "no packages being tested depend on"; \
    done
	$(gobin_tool) -run $(gocovmerge) cover/*.out > cover/all.merged
ifndef CI
	@echo "Coverage Report..."
	go tool cover -html .GOPATH/cover/all.merged
else
	@echo "Coverage Report In CI..."
	go tool cover -html .GOPATH/cover/all.merged -o .GOPATH/cover/all.html
endif
	@echo ""
	@echo "=====> Total test coverage: <====="
	@echo ""
	go tool cover -func .GOPATH/cover/all.merged

format: goimports
	@echo "Formatting..."
	find . -iname \*.go | grep -v \
        -e "^$$" $(addprefix -e ,$(IGNORED_PACKAGES)) | xargs $(gobin_tool) -run $(goimports) -w

##### =====> Internals <===== #####

.PHONY: setup
setup: goimports gocovmerge goreleaser gox clean
	env
	@echo "Setup..."
	if ! grep "/bin" .gitignore > /dev/null 2>&1; then \
        echo "/bin" >> .gitignore; \
    fi
	if ! grep "/cover" .gitignore > /dev/null 2>&1; then \
        echo "/cover" >> .gitignore; \
    fi
	mkdir -p cover
	mkdir -p bin
	mkdir -p test

VERSION          := $(shell git describe --tags --always --dirty="-dev")
DATE             := $(shell date -u '+%Y-%m-%d-%H:%M UTC')
VERSION_FLAGS    := -ldflags='-X "main.buildVersion=$(VERSION)" -X "main.buildTime=$(DATE)"'

TESTDIRS ?= ./...

.PHONY: goimports
goimports: $(gobin_tool)
	$(gobin_tool) -d $(goimports)

.PHONY: gocovmerge
gocovmerge: $(gobin_tool)
	$(gobin_tool) -d $(gocovmerge)

.PHONY: goreleaser
goreleaser: $(gobin_tool)
	$(gobin_tool) -d $(goreleaser)

.PHONY: gox
gox: $(gobin_tool)
	# We make gox globally available, for people to use by hand
	$(gobin_tool) $(gox)

$(gobin_tool):
	GO111MODULE=off go get -u github.com/myitcv/gobin

authors:
	git log --all --format='%aN <%cE>' | sort -u  | sed -n '/github/!p' > GITAUTHORS
	cat AUTHORS GITAUTHORS  | sort -u > NEWAUTHORS
	mv NEWAUTHORS AUTHORS
	rm -f NEWAUTHORS
	rm -f GITAUTHORS

SETUP_ENVTEST_VERSION ?= v0.0.0-20250604165838-d6126d850224
ENVTEST_K8S_VERSION := 1.31.x

ENVTEST ?= go run sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION)
ENVTEST_DIR ?= $(shell pwd)/.envtest
export KUBEBUILDER_ASSETS ?= $(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(ENVTEST_DIR) -p path)

.PHONY: envtest
envtest:
	# You can add klog flags for debugging, like: -klog.v=10 -klog.logtostderr
	# klogv2 flags just wraps our existing logrus.
	$(GOTEST) -run=TestEnvtest ./node -envtest=true

.PHONY: fmt
fmt:
	goimports -w $(shell go list -f '{{.Dir}}' ./...)


export GOLANG_CI_LINT_VERSION ?= v1.49.0
DOCKER_BUILD ?= docker buildx build

.PHONY: lint
lint:
	$(DOCKER_BUILD) --target=lint --build-arg GOLANG_CI_LINT_VERSION --build-arg OUT_FORMAT .
