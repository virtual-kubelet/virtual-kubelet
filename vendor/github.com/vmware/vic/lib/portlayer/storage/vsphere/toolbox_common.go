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

package vsphere

import (
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi/guest"
	"github.com/vmware/govmomi/guest/toolbox"
	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/tether/shared"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	VixEToolsNotRunning = "(3016, 0)"
)

var (
	toolboxRetryConf *retry.BackoffConfig
)

func init() {
	toolboxRetryConf = retry.NewBackoffConfig()

	// These numbers are somewhat arbitrary best guesses
	toolboxRetryConf.MaxElapsedTime = time.Second * 30
	toolboxRetryConf.InitialInterval = time.Millisecond * 500
	toolboxRetryConf.MaxInterval = time.Second * 5
}

// Parse Archive builds an archive url with disklabel, filtersec, recursive, and data booleans.
func BuildArchiveURL(op trace.Operation, disklabel, target string, fs *archive.FilterSpec, recurse, data bool) (string, error) {
	encodedSpec, err := archive.EncodeFilterSpec(op, fs)
	if err != nil {
		return "", err
	}
	target = path.Join("/archive:/", target)

	// if diskLabel is longer than 16 characters, then the function was passed a containerID
	// use containerfs as the diskLabel
	if len(disklabel) > 16 {
		disklabel = "containerfs"
	}

	// note that the query parameters a SkipX for recurse and data so values are inverted
	target += "?" + (url.Values{
		shared.DiskLabelQueryName:   []string{disklabel},
		shared.FilterSpecQueryName:  []string{*encodedSpec},
		shared.SkipRecurseQueryName: []string{strconv.FormatBool(!recurse)},
		shared.SkipDataQueryName:    []string{strconv.FormatBool(!data)},
	}).Encode()

	op.Debugf("OnlineData* Url: %s", target)
	return target, nil
}

// GetToolboxClient returns a toolbox client given a vm and id
func GetToolboxClient(op trace.Operation, vm *vm.VirtualMachine, id string) (*toolbox.Client, error) {
	opmgr := guest.NewOperationsManager(vm.Session.Client.Client, vm.Reference())
	pm, err := opmgr.ProcessManager(op)
	if err != nil {
		op.Debugf("Failed to create new process manager ")
		return nil, err
	}
	fm, err := opmgr.FileManager(op)
	if err != nil {
		op.Debugf("Failed to create new file manager ")
		return nil, err
	}

	return &toolbox.Client{
		ProcessManager: pm,
		FileManager:    fm,
		Authentication: &types.NamePasswordAuthentication{
			Username: id,
		},
	}, nil
}

// isInvalidStateError is used to identify whether the supplied error is an InvalidState fault
func isInvalidStateError(err error) bool {
	if soap.IsSoapFault(err) {
		switch soap.ToSoapFault(err).VimFault().(type) {
		case types.InvalidState:
			return true
		}
	}

	if soap.IsVimFault(err) {
		switch soap.ToVimFault(err).(type) {
		case *types.InvalidState:
			return true
		}
	}

	switch err := err.(type) {
	case task.Error:
		switch err.Fault().(type) {
		case *types.InvalidState:
			return true
		}
	}
	return false
}

// IsToolBoxConflictErr checks for conflictError for online import
func IsToolBoxStateChangeErr(err error) bool {
	// check if error has to do with toolbox state changes
	if soap.IsSoapFault(err) {
		switch soap.ToSoapFault(err).VimFault().(type) {
		case types.InvalidState:
			return true
		case types.InvalidPowerState:
			return true
		case types.GuestOperationsUnavailable:
			return true
		case types.SystemError:
			return strings.Contains(err.Error(), VixEToolsNotRunning)
		}
	}

	switch err.(type) {
	case *url.Error:
		err = err.(*url.Error).Err
		switch err.(type) {
		case *net.OpError:
			// can check for error message as well
			return true
		}
	}

	// NOTE: on certain failures toolbox only returns 500 which can be caused by state change in the middle
	// but can also be caused by invalid command. There is no way to tell unless toolbox returns more information.
	return false
}
