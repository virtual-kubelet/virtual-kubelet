// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package tether

import (
	"io"

	"golang.org/x/crypto/ssh"
)

type ContainerSigner struct {
	id string
}

func (c *ContainerSigner) PublicKey() ssh.PublicKey {
	return *c
}

// we're going to ignore everything for the moment as we're repurposing the host key for the id.
// later we may use a genuine host key and an SSH out-of-band request to get the container id.
func (c *ContainerSigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	return &ssh.Signature{
		Format: "container-id",
		Blob:   []byte{},
	}, nil
}

func (c ContainerSigner) Type() string {
	return "container-id"
}

func (c ContainerSigner) Marshal() []byte {
	return []byte(c.id)
}

func (c ContainerSigner) Verify(data []byte, sig *ssh.Signature) error {
	return nil
}

func NewSigner(id string) *ContainerSigner {
	signer := &ContainerSigner{
		id: id,
	}

	return signer
}
