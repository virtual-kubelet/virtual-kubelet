// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/trace"
)

// testName allows easy embedding of the test function name in the test body as string data
func testName(t *testing.T) string {
	pc, _, _, ok := runtime.Caller(1)
	require.True(t, ok, "unable to determine test name")

	// lets only return the func name from the repo (vic)
	// down - i.e. vic/lib/etc vs. github.com/vmware/vic/lib/etc
	// if github.com/vmware doesn't match then the original is returned
	pkgAndFunc := path.Base(runtime.FuncForPC(pc).Name())
	parts := strings.Split(pkgAndFunc, ".")
	return parts[len(parts)-1]
}

func TestServiceUrl(t *testing.T) {
	DefaultHost, _ = url.Parse("http://foo.com/")
	u := ServiceURL(StorageURLPath)

	if !assert.Equal(t, "http://foo.com/storage", u.String()) {
		return
	}
}

func TestNameConventionDefault(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "%s-%s"
	template := fmt.Sprintf(formatString, config.NameToken.String(), config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name, shortID), formatted, "display name not as expected")

	second := DisplayName(op, cfg, template)
	assert.Equal(t, formatted, second, "second call should return the same output")
}

func TestNameConventionUnspecified(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "%s-%s"
	template := ""

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name, shortID), formatted, "display name not as expected")
}

func TestNameConventionNameInsertOnly(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "%s"
	template := fmt.Sprintf(formatString, config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name), formatted, "display name not as expected")
}

func TestNameConventionIdInsertOnly(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "%s"
	template := fmt.Sprintf(formatString, config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, shortID), formatted, "display name not as expected")
}

func TestNameConventionNamePre(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "pre-%s"
	template := fmt.Sprintf(formatString, config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name), formatted, "display name not as expected")
}

func TestNameConventionNamePost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "%s-post"
	template := fmt.Sprintf(formatString, config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name), formatted, "display name not as expected")
}

func TestNameConventionIDPre(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "pre-%s"
	template := fmt.Sprintf(formatString, config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, shortID), formatted, "display name not as expected")

}

func TestNameConventionIDPost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "%s-post"
	template := fmt.Sprintf(formatString, config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, shortID), formatted, "display name not as expected")
}

func TestNameConventionNameBoth(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "pre-%s-post"
	template := fmt.Sprintf(formatString, config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name), formatted, "display name not as expected")
}

func TestNameConventionIDBoth(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "pre-%s-post"
	template := fmt.Sprintf(formatString, config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, shortID), formatted, "display name not as expected")
}

func TestNameConventionNameAndIDWithPrePost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "pre-%s-%s-post"
	template := fmt.Sprintf(formatString, config.NameToken.String(), config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, cfg.Name, shortID), formatted, "display name not as expected")
}

func TestNameConventionIDAndNameWithPrePost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "pre-%s-%s-post"
	template := fmt.Sprintf(formatString, config.IDToken.String(), config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, fmt.Sprintf(formatString, shortID, cfg.Name), formatted, "display name not as expected")
}

func TestNameConventionNameOverflowWithPrePost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "this-is-a-really-long-pre-segment-in-order-to-test-name-truncation-%s-post"
	template := fmt.Sprintf(formatString, config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, constants.MaxVMNameLength, len(formatted), "name was expected to be the max vm name length")
}

func TestNameConventionIDOverflowWithPrePost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "this-is-a-really-long-pre-segment-in-order-to-test-name-truncation-%s-post"
	template := fmt.Sprintf(formatString, config.IDToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, constants.MaxVMNameLength, len(formatted), "name was expected to be the max vm name length")
}

func TestNameConventionBothOverflowWithPrePost(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "this-is-a-really-long-pre-segment-in-order-to-test-name-truncation-%s-%s-post"
	template := fmt.Sprintf(formatString, config.IDToken.String(), config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	_ = cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	assert.Equal(t, constants.MaxVMNameLength, len(formatted), "name was expected to be the max vm name length")
}

func TestTemplateOverflow(t *testing.T) {
	op := trace.NewOperation(context.Background(), testName(t))

	formatString := "this-is-a-really-long-pre-segment-in-order-to-test-name-truncation-that-overflows-without-any-%s-post"
	template := fmt.Sprintf(formatString, config.IDToken.String(), config.NameToken.String())

	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   "abcdefg0123456789hijklmnopqrstu",
		Name: testName(t),
	}

	shortID := cfg.ID[:constants.ShortIDLen]

	formatted := DisplayName(op, cfg, template)
	// behaviour of template overflow is to revert to default template
	assert.Equal(t, fmt.Sprintf("%s-%s", cfg.Name, shortID), formatted, "display name not as expected")
}
