package azure

import (
	"bytes"
	"strings"
	"testing"
)

const cfg = `
Region = "westus"
ResourceGroup = "virtual-kubeletrg"
CPU = "100"
Memory = "100Gi"
Pods = "20"`

func TestConfig(t *testing.T) {
	br := bytes.NewReader([]byte(cfg))
	var p ACIProvider
	err := p.loadConfig(br)
	if err != nil {
		t.Error(err)
	}
	wanted := "westus"
	if p.region != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, p.region)
	}

	wanted = "virtual-kubeletrg"
	if p.resourceGroup != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, p.resourceGroup)
	}

	wanted = "100"
	if p.cpu != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, p.cpu)
	}

	wanted = "100Gi"
	if p.memory != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, p.memory)
	}

	wanted = "20"
	if p.pods != wanted {
		t.Errorf("Wanted %s, got %s.", wanted, p.pods)
	}
}

const cfgBad = `
Region = "westus"
ResourceGroup = "virtual-kubeletrg"
OperatingSystem = "noop"`

func TestBadConfig(t *testing.T) {
	br := bytes.NewReader([]byte(cfgBad))
	var p ACIProvider
	err := p.loadConfig(br)
	if err == nil {
		t.Fatal("expected loadConfig to fail with bad operating system option")
	}

	if !strings.Contains(err.Error(), "is not a valid operating system") {
		t.Fatalf("expected loadConfig to fail with 'is not a valid operating system' but got: %v", err)

	}
}

const defCfg = `
Region = "westus"
ResourceGroup = "virtual-kubeletrg"`

func TestDefaultedConfig(t *testing.T) {
	br := bytes.NewReader([]byte(defCfg))
	var p ACIProvider
	err := p.loadConfig(br)
	if err != nil {
		t.Error(err)
	}
	// Test that defaults work with no settings in config.
	wanted := "20"
	if p.cpu != wanted {
		t.Errorf("Wanted default %s, got %s.", wanted, p.cpu)
	}

	wanted = "100Gi"
	if p.memory != wanted {
		t.Errorf("Wanted default %s, got %s.", wanted, p.memory)
	}

	wanted = "20"
	if p.pods != wanted {
		t.Errorf("Wanted default %s, got %s.", wanted, p.pods)
	}
}
