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
package admiral

import (
	"context"
	"flag"
	"net/url"
	"os"
	"testing"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/vsphere/session"
)

var (
	target     *string
	user       *string
	password   *string
	thumbprint *string
	vchID      *string
)

func init() {
	thumbprint = flag.String("thumbprint", "", "ESX/VC thumbprint")
	target = flag.String("target", "", "ESX/VC target")
	user = flag.String("user", "root", "ESX/VC username")
	password = flag.String("password", "", "ESX/VC password")
	vchID = flag.String("vch", "", "VCH docker endpoint")
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestGetConfig(t *testing.T) {
	if target == nil || *target == "" {
		t.SkipNow()
	}

	log.SetLevel(log.DebugLevel)
	sess := session.NewSession(&session.Config{
		Service:    *target,
		User:       url.UserPassword(*user, *password),
		Insecure:   true,
		Thumbprint: *thumbprint,
	})

	ctx := context.Background()
	sess.Connect(ctx)

	src := NewSource(sess, *vchID)
	cfg, err := src.Get(ctx)
	if err != nil {
		log.Errorf("error: %s", err)
		os.Exit(1)
	}

	log.Infof("%+v", cfg)
}
