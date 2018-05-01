// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package validate

import (
	"context"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

// MigrateConfig migrate old VCH configuration to new version. Currently check required fields only
func (v *Validator) ValidateMigratedConfig(ctx context.Context, conf *config.VirtualContainerHostConfigSpec) (*config.VirtualContainerHostConfigSpec, error) {
	defer trace.End(trace.Begin(conf.Name, ctx))

	v.assertTarget(ctx, conf)
	v.assertDatastore(ctx, conf)
	v.assertNetwork(ctx, conf)
	if err := v.ListIssues(ctx); err != nil {
		return conf, err
	}
	return v.migrateData(ctx, conf)
}

func (v *Validator) migrateData(ctx context.Context, conf *config.VirtualContainerHostConfigSpec) (*config.VirtualContainerHostConfigSpec, error) {
	conf.Version = version.GetBuild()
	return conf, nil
}

func (v *Validator) assertNetwork(ctx context.Context, conf *config.VirtualContainerHostConfigSpec) {
	// minimum network configuration check
}

// assertDatastore check required datastore configuration only
func (v *Validator) assertDatastore(ctx context.Context, conf *config.VirtualContainerHostConfigSpec) {
	defer trace.End(trace.Begin("", ctx))
	if len(conf.ImageStores) == 0 {
		v.NoteIssue(errors.New("Image store is not set"))
	}
}

func (v *Validator) assertTarget(ctx context.Context, conf *config.VirtualContainerHostConfigSpec) {
	defer trace.End(trace.Begin("", ctx))

	if conf.Target == "" {
		v.NoteIssue(errors.New("target is not set"))
	}

	if conf.Username == "" {
		v.NoteIssue(errors.New("target username is not set"))
	}

	if conf.Token == "" {
		v.NoteIssue(errors.New("target token is not set"))
	}
}

func (v *Validator) AssertVersion(ctx context.Context, conf *config.VirtualContainerHostConfigSpec) (err error) {
	defer trace.End(trace.Begin("", ctx))
	defer func() {
		err = v.ListIssues(ctx)
	}()

	if conf.Version == nil {
		v.NoteIssue(errors.Errorf("Unknown version of VCH %q", conf.Name))
		return err
	}
	var older bool
	installerBuild := version.GetBuild()
	if older, err = conf.Version.IsOlder(installerBuild); err != nil {
		v.NoteIssue(errors.Errorf("Failed to compare VCH version %q with installer version %q: %s", conf.Version.ShortVersion(), installerBuild.ShortVersion(), err))
		return err
	}
	if !older {
		v.NoteIssue(errors.Errorf("%q has same or newer version %s than installer version %s. No upgrade is available.", conf.Name, conf.Version.ShortVersion(), installerBuild.ShortVersion()))
		return err
	}
	return nil
}
