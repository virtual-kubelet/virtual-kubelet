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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/dns"
	vlog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/version"
)

var (
	options = dns.ServerOptions{}
)

func init() {
	flag.StringVar(&options.IP, "ip", dns.DefaultIP, "IP to use")
	flag.IntVar(&options.Port, "port", dns.DefaultPort, "Port to bind")
	flag.StringVar(&options.Interface, "interface", "", "Interface to bind")

	flag.Var(&options.Nameservers, "nameservers", "Nameservers to use")

	flag.DurationVar(&options.Timeout, "timeout", dns.DefaultTimeout, "Timeout for external DNS queries")

	flag.DurationVar(&options.TTL, "ttl", dns.DefaultTTL, "TTL")
	flag.IntVar(&options.CacheSize, "cachesize", dns.DefaultCacheSize, "Cache size to use")

	flag.BoolVar(&options.Debug, "debug", false, "Enable debugging")

	flag.Parse()
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, fmt.Sprintf("%s : %s", r, debug.Stack()))
		}
	}()

	if version.Show() {
		// #nosec: Errors unhandled.
		fmt.Fprintf(os.Stdout, "%s\n", version.String())
		return
	}

	// Initiliaze logger with default TextFormatter
	log.SetFormatter(vlog.NewTextFormatter())

	// Set the log level
	if options.Debug {
		log.SetLevel(log.DebugLevel)
	}

	server := dns.NewServer(options)
	if server != nil {
		server.Start()
	}

	// handle the signals and gracefully shutdown the server
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Warnf("signal %s received", <-sig)
		server.Stop()
	}()

	server.Wait()
}
