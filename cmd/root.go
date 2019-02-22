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
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/register"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet"
	octrace "go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultDaemonPort = "10250"
	// kubeSharedInformerFactoryDefaultResync is the default resync period used by the shared informer factories for Kubernetes resources.
	// It is set to the same value used by the Kubelet, and can be overridden via the "--full-resync-period" flag.
	// https://github.com/kubernetes/kubernetes/blob/v1.12.2/pkg/kubelet/apis/config/v1beta1/defaults.go#L51
	kubeSharedInformerFactoryDefaultResync = 1 * time.Minute
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
var taint *corev1.Taint
var k8sClient *kubernetes.Clientset
var p providers.Provider
var rm *manager.ResourceManager
var apiConfig *apiServerConfig
var podInformer corev1informers.PodInformer
var kubeSharedInformerFactoryResync time.Duration
var podSyncWorkers int

var userTraceExporters []string
var userTraceConfig = TracingExporterOptions{Tags: make(map[string]string)}
var traceSampler string

// Create a root context to be used by the pod controller and by the shared informer factories.
var rootContext, rootContextCancel = context.WithCancel(context.Background())

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "virtual-kubelet",
	Short: "virtual-kubelet provides a virtual kubelet interface for your kubernetes cluster.",
	Long: `virtual-kubelet implements the Kubelet interface with a pluggable
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
	Run: func(cmd *cobra.Command, args []string) {
		defer rootContextCancel()

		vk := vkubelet.New(vkubelet.Config{
			Client:          k8sClient,
			Namespace:       kubeNamespace,
			NodeName:        nodeName,
			Taint:           taint,
			Provider:        p,
			ResourceManager: rm,
			PodSyncWorkers:  podSyncWorkers,
			PodInformer:     podInformer,
		})

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sig
			rootContextCancel()
		}()

		cancelHTTP, err := setupHTTPServer(rootContext, apiConfig)
		if err != nil {
			log.G(rootContext).Fatal(err)
		}
		defer cancelHTTP()

		if err := vk.Run(rootContext); err != nil && errors.Cause(err) != context.Canceled {
			log.G(rootContext).Fatal(err)
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

type mapVar map[string]string

func (mv mapVar) String() string {
	var s string
	for k, v := range mv {
		if s == "" {
			s = fmt.Sprintf("%s=%v", k, v)
		} else {
			s += fmt.Sprintf(", %s=%v", k, v)
		}
	}
	return s
}

func (mv mapVar) Set(s string) error {
	split := strings.SplitN(s, "=", 2)
	if len(split) != 2 {
		return errors.Errorf("invalid format, must be `key=value`: %s", s)
	}

	_, ok := mv[split[0]]
	if ok {
		return errors.Errorf("duplicate key: %s", split[0])
	}
	mv[split[0]] = split[1]
	return nil
}

func (mv mapVar) Type() string {
	return "map"
}

func init() {
	// make sure the default logger/tracer is initialized
	log.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	trace.T = opencensus.Adapter{}

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
	RootCmd.PersistentFlags().IntVar(&podSyncWorkers, "pod-sync-workers", 10, `set the number of pod synchronization workers`)

	RootCmd.PersistentFlags().StringSliceVar(&userTraceExporters, "trace-exporter", nil, fmt.Sprintf("sets the tracing exporter to use, available exporters: %s", AvailableTraceExporters()))
	RootCmd.PersistentFlags().StringVar(&userTraceConfig.ServiceName, "trace-service-name", "virtual-kubelet", "sets the name of the service used to register with the trace exporter")
	RootCmd.PersistentFlags().Var(mapVar(userTraceConfig.Tags), "trace-tag", "add tags to include with traces in key=value form")
	RootCmd.PersistentFlags().StringVar(&traceSampler, "trace-sample-rate", "", "set probability of tracing samples")

	RootCmd.PersistentFlags().DurationVar(&kubeSharedInformerFactoryResync, "full-resync-period", kubeSharedInformerFactoryDefaultResync, "how often to perform a full resync of pods between kubernetes and the provider")

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

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.G(context.TODO()).WithField("logLevel", logLevel).Fatal("log level is not supported")
	}

	logrus.SetLevel(level)
	logger := logruslogger.FromLogrus(logrus.WithFields(logrus.Fields{
		"provider":        provider,
		"operatingSystem": operatingSystem,
		"node":            nodeName,
		"namespace":       kubeNamespace,
	}))

	rootContext = log.WithLogger(rootContext, logger)

	log.L = logger

	if !disableTaint {
		taint, err = getTaint(taintKey, provider)
		if err != nil {
			logger.WithError(err).Fatal("Error setting up desired kubernetes node taint")
		}
	}

	k8sClient, err = newClient(kubeConfig)
	if err != nil {
		logger.WithError(err).Fatal("Error creating kubernetes client")
	}

	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(k8sClient, kubeSharedInformerFactoryResync, kubeinformers.WithNamespace(kubeNamespace), kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	}))
	// Create a pod informer so we can pass its lister to the resource manager.
	podInformer = podInformerFactory.Core().V1().Pods()

	// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
	scmInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(k8sClient, kubeSharedInformerFactoryResync)
	// Create a secret informer and a config map informer so we can pass their listers to the resource manager.
	secretInformer := scmInformerFactory.Core().V1().Secrets()
	configMapInformer := scmInformerFactory.Core().V1().ConfigMaps()

	// Create a new instance of the resource manager that uses the listers above for pods, secrets and config maps.
	rm, err = manager.NewResourceManager(podInformer.Lister(), secretInformer.Lister(), configMapInformer.Lister())
	if err != nil {
		logger.WithError(err).Fatal("Error initializing resource manager")
	}

	// Start the shared informer factory for pods.
	go podInformerFactory.Start(rootContext.Done())
	// Start the shared informer factory for secrets and configmaps.
	go scmInformerFactory.Start(rootContext.Done())

	daemonPortEnv := getEnv("KUBELET_PORT", defaultDaemonPort)
	daemonPort, err := strconv.ParseInt(daemonPortEnv, 10, 32)
	if err != nil {
		logger.WithError(err).WithField("value", daemonPortEnv).Fatal("Invalid value from KUBELET_PORT in environment")
	}

	initConfig := register.InitConfig{
		ConfigPath:      providerConfig,
		NodeName:        nodeName,
		OperatingSystem: operatingSystem,
		ResourceManager: rm,
		DaemonPort:      int32(daemonPort),
		InternalIP:      os.Getenv("VKUBELET_POD_IP"),
	}

	p, err = register.GetProvider(provider, initConfig)
	if err != nil {
		logger.WithError(err).Fatal("Error initializing provider")
	}

	apiConfig, err = getAPIConfig(metricsAddr)
	if err != nil {
		logger.WithError(err).Fatal("Error reading API config")
	}

	if podSyncWorkers <= 0 {
		logger.Fatal("The number of pod synchronization workers should not be negative")
	}

	for k := range userTraceConfig.Tags {
		if reservedTagNames[k] {
			logger.WithField("tag", k).Fatal("must not use a reserved tag key")
		}
	}
	userTraceConfig.Tags["operatingSystem"] = operatingSystem
	userTraceConfig.Tags["provider"] = provider
	userTraceConfig.Tags["nodeName"] = nodeName
	for _, e := range userTraceExporters {
		if e == "zpages" {
			go setupZpages()
			continue
		}
		exporter, err := GetTracingExporter(e, userTraceConfig)
		if err != nil {
			log.L.WithError(err).WithField("exporter", e).Fatal("Cannot initialize exporter")
		}
		octrace.RegisterExporter(exporter)
	}
	if len(userTraceExporters) > 0 {
		var s octrace.Sampler
		switch strings.ToLower(traceSampler) {
		case "":
		case "always":
			s = octrace.AlwaysSample()
		case "never":
			s = octrace.NeverSample()
		default:
			rate, err := strconv.Atoi(traceSampler)
			if err != nil {
				logger.WithError(err).WithField("rate", traceSampler).Fatal("unsupported trace sample rate, supported values: always, never, or number 0-100")
			}
			if rate < 0 || rate > 100 {
				logger.WithField("rate", traceSampler).Fatal("trace sample rate must not be less than zero or greater than 100")
			}
			s = octrace.ProbabilitySampler(float64(rate) / 100)
		}

		if s != nil {
			octrace.ApplyConfig(
				octrace.Config{
					DefaultSampler: s,
				},
			)
		}
	}
}
