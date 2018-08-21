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

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	vkubelet "github.com/virtual-kubelet/virtual-kubelet/vkubelet"
	corev1 "k8s.io/api/core/v1"
)

var kubeletConfig string
var kubeConfig string
var kubeNamespace string
var nodeName string
var operatingSystem string
var provider string
var providerConfig string
var taintKey string
var disableTaint bool
var logLevel string
var metricsAddr string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "virtual-kubelet",
	Short: "virtual-kubelet provides a virtual kubelet interface for your kubernetes cluster.",
	Long: `virtual-kubelet implements the Kubelet interface with a pluggable 
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
	Run: func(cmd *cobra.Command, args []string) {
		f, err := vkubelet.New(nodeName, operatingSystem, kubeNamespace, kubeConfig, provider, providerConfig, taintKey, disableTaint, metricsAddr)
		if err != nil {
			log.L.WithError(err).Fatal("Error initializing virtual kubelet")
		}
		if err := f.Run(context.Background()); err != nil {
			log.L.Fatal(err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.GetLogger(context.TODO()).WithError(err).Fatal("Error executing root command")
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// read default node name from environment variable.
	// it can be overwritten by cli flags if specified.
	defaultNodeName := os.Getenv("DEFAULT_NODE_NAME")
	if defaultNodeName == "" {
		defaultNodeName = "virtual-kubelet"
	}
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	//RootCmd.PersistentFlags().StringVar(&kubeletConfig, "config", "", "config file (default is $HOME/.virtual-kubelet.yaml)")
	RootCmd.PersistentFlags().StringVar(&kubeConfig, "kubeconfig", "", "config file (default is $HOME/.kube/config)")
	RootCmd.PersistentFlags().StringVar(&kubeNamespace, "namespace", "", "kubernetes namespace (default is 'all')")
	RootCmd.PersistentFlags().StringVar(&nodeName, "nodename", defaultNodeName, "kubernetes node name")
	RootCmd.PersistentFlags().StringVar(&operatingSystem, "os", "Linux", "Operating System (Linux/Windows)")
	RootCmd.PersistentFlags().StringVar(&provider, "provider", "", "cloud provider")
	RootCmd.PersistentFlags().BoolVar(&disableTaint, "disable-taint", false, "disable the virtual-kubelet node taint")
	RootCmd.PersistentFlags().StringVar(&providerConfig, "provider-config", "", "cloud provider configuration file")
	RootCmd.PersistentFlags().StringVar(&metricsAddr, "metrics-addr", ":10255", "address to listen for metrics/stats requests")

	RootCmd.PersistentFlags().StringVar(&taintKey, "taint", "", "Set node taint key")
	RootCmd.PersistentFlags().MarkDeprecated("taint", "Taint key should now be configured using the VK_TAINT_KEY environment variable")
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", `set the log level, e.g. "trace", debug", "info", "warn", "error"`)

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if provider == "" {
		log.G(context.TODO()).Fatal("You must supply a cloud provider option: use --provider")
	}

	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		log.G(context.TODO()).WithError(err).Fatal("Error reading homedir")
	}

	if kubeletConfig != "" {
		// Use config file from the flag.
		viper.SetConfigFile(kubeletConfig)
	} else {
		// Search config in home directory with name ".virtual-kubelet" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".virtual-kubelet")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.G(context.TODO()).Debugf("Using config file %s", viper.ConfigFileUsed())
	}

	if kubeConfig == "" {
		kubeConfig = filepath.Join(home, ".kube", "config")

	}

	if kubeNamespace == "" {
		kubeNamespace = corev1.NamespaceAll
	}

	// Validate operating system.
	ok, _ := providers.ValidOperatingSystems[operatingSystem]
	if !ok {
		log.G(context.TODO()).WithField("OperatingSystem", operatingSystem).Fatalf("Operating system not supported. Valid options are: %s", strings.Join(providers.ValidOperatingSystems.Names(), " | "))
	}

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.G(context.TODO()).WithField("logLevel", logLevel).Fatal("log level is not supported")
	}

	logger := log.L.WithFields(logrus.Fields{
		"provider":        provider,
		"operatingSystem": operatingSystem,
		"node":            nodeName,
		"namespace":       kubeNamespace,
	})
	logger.Level = level
	log.L = logger
}
