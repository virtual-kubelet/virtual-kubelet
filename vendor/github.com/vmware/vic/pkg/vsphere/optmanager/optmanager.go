// Copyright 2017 VMware, Inc. All Rights Reserved.
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

// Package optmanager provides govmomi helpers for the OptionManager.
package optmanager

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// QueryOptionValue uses the session client and OptionManager to look up the input option
// and return the received value.
func QueryOptionValue(ctx context.Context, s *session.Session, option string) (string, error) {
	client := s.Vim25()
	optMgr := object.NewOptionManager(client, *client.ServiceContent.Setting)
	opts, err := optMgr.Query(ctx, option)
	if err != nil {
		return "", fmt.Errorf("error querying option %q: %s", option, err)
	}

	if len(opts) == 1 {
		return fmt.Sprintf("%v", opts[0].GetOptionValue().Value), nil
	}

	return "", fmt.Errorf("%d values querying option %q", len(opts), option)
}

// UpdateOptionValue uses the session client and OptionManager to set the input option
func UpdateOptionValue(ctx context.Context, s *session.Session, option string, value string) error {
	client := s.Vim25()
	optMgr := object.NewOptionManager(client, *client.ServiceContent.Setting)
	var opts []types.BaseOptionValue
	opts = append(opts, &types.OptionValue{
		Key:   option,
		Value: value,
	})
	err := optMgr.Update(ctx, opts)
	if err != nil {
		return fmt.Errorf("error setting option %q: %s", option, err)
	}
	return nil
}
