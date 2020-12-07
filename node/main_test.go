/* This file is just a place for the TestMain override function to live, plus whatever custom flags we are interested in */
package node

import (
	"flag"
	"os"
	"testing"

	klogv1 "k8s.io/klog"
)

var enableEnvTest = flag.Bool("envtest", false, "Enable envtest based tests")

func TestMain(m *testing.M) {
	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klogv1.InitFlags(flagset)
	flagset.VisitAll(func(f *flag.Flag) {
		flag.Var(f.Value, "klog."+f.Name, f.Usage)
	})
	flag.Parse()
	os.Exit(m.Run())
}
