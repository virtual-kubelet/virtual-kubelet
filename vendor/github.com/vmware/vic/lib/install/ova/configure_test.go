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

package ova

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/vsphere/session"
)

func TestGetOvaVMByTagBadURL(t *testing.T) {
	ctx := context.Background()
	bogusURL := "foo/bar.url://what-is-this"
	vm, err := getOvaVMByTag(ctx, nil, bogusURL)
	assert.Nil(t, vm)
	assert.Error(t, err)
}

func TestGetOvaVMByTag(t *testing.T) {
	username := os.Getenv("TEST_VC_USERNAME")
	password := os.Getenv("TEST_VC_PASSWORD")
	vcURL := os.Getenv("TEST_VC_URL")
	ovaURL := os.Getenv("TEST_OVA_URL")

	if vcURL == "" || ovaURL == "" {
		log.Infof("Skipping TestGetOvaVMByTag")
		t.Skipf("This test should only run against a VC with a deployed OVA")
	}

	ctx := context.Background()

	vc, err := url.Parse(vcURL)
	if err != nil {
		fmt.Printf("Failed to parse VC url: %s", err)
		t.FailNow()
	}

	vc.User = url.UserPassword(username, password)

	var cert object.HostCertificateInfo
	if err = cert.FromURL(vc, new(tls.Config)); err != nil {
		log.Error(err.Error())
		t.FailNow()
	}

	if cert.Err != nil {
		log.Errorf("Failed to verify certificate for target=%s (thumbprint=%s)", vc.Host, cert.ThumbprintSHA1)
		log.Error(cert.Err.Error())
	}

	tp := cert.ThumbprintSHA1
	log.Infof("Accepting host %q thumbprint %s", vc.Host, tp)

	sessionConfig := &session.Config{
		Thumbprint:     tp,
		Service:        vc.String(),
		DatacenterPath: "/ha-datacenter",
		DatastorePath:  "datastore1",
		User:           vc.User,
		Insecure:       true,
	}

	s := session.NewSession(sessionConfig)
	sess, err := s.Connect(ctx)
	if err != nil {
		log.Errorf("Error connecting: %s", err.Error())
	}
	defer sess.Logout(ctx)

	sess, err = sess.Populate(ctx)
	if err != nil {
		log.Errorf("Error populating: %s", err.Error())
	}

	vm, err := getOvaVMByTag(ctx, sess, ovaURL)
	if err != nil {
		log.Errorf("Error getting OVA by tag: %s", err.Error())
	}
	if vm == nil {
		log.Errorf("No VM found")
		t.FailNow()
	}

	log.Infof("%s", vm.String())
}
