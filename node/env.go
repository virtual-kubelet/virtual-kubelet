// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import "github.com/virtual-kubelet/virtual-kubelet/podutils"

// These consts have been moved into the podutils package. Aliases are preserved here
// to ensure users references don't break
const (
	ReasonOptionalConfigMapNotFound       = podutils.ReasonOptionalConfigMapNotFound
	ReasonOptionalConfigMapKeyNotFound    = podutils.ReasonOptionalConfigMapKeyNotFound
	ReasonFailedToReadOptionalConfigMap   = "FailedToReadOptionalConfigMap"
	ReasonOptionalSecretNotFound          = podutils.ReasonOptionalSecretNotFound
	ReasonOptionalSecretKeyNotFound       = podutils.ReasonOptionalSecretKeyNotFound
	ReasonFailedToReadOptionalSecret      = "FailedToReadOptionalSecret"
	ReasonMandatoryConfigMapNotFound      = podutils.ReasonMandatoryConfigMapNotFound
	ReasonMandatoryConfigMapKeyNotFound   = podutils.ReasonMandatoryConfigMapKeyNotFound
	ReasonFailedToReadMandatoryConfigMap  = podutils.ReasonFailedToReadMandatoryConfigMap
	ReasonMandatorySecretNotFound         = podutils.ReasonMandatorySecretNotFound
	ReasonMandatorySecretKeyNotFound      = podutils.ReasonMandatorySecretKeyNotFound
	ReasonFailedToReadMandatorySecret     = podutils.ReasonFailedToReadMandatorySecret
	ReasonInvalidEnvironmentVariableNames = podutils.ReasonInvalidEnvironmentVariableNames
)
