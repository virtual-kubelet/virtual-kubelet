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

package root

import (
	"fmt"
	"os"
	"time"
)

type apiServerConfig struct {
	CertPath              string
	KeyPath               string
	CACertPath            string
	Addr                  string
	MetricsAddr           string
	StreamIdleTimeout     time.Duration
	StreamCreationTimeout time.Duration
}

func getAPIConfig(c Opts) (*apiServerConfig, error) {
	config := apiServerConfig{
		CertPath:   os.Getenv("APISERVER_CERT_LOCATION"),
		KeyPath:    os.Getenv("APISERVER_KEY_LOCATION"),
		CACertPath: os.Getenv("APISERVER_CA_CERT_LOCATION"),
	}

	config.Addr = fmt.Sprintf(":%d", c.ListenPort)
	config.MetricsAddr = c.MetricsAddr
	config.StreamIdleTimeout = c.StreamIdleTimeout
	config.StreamCreationTimeout = c.StreamCreationTimeout

	return &config, nil
}
