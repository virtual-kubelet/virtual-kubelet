package aci

import (
	"context"
	"testing"
)

func TestGetResourceProviderMetadata(t *testing.T) {
	metadata, err := client.GetResourceProviderMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if metadata.VNetSupportRegions == nil || len(metadata.VNetSupportRegions) == 0 {
		t.Fatal("VNet support regions should not be empty")
	}

	if metadata.GPUSupportRegions == nil || len(metadata.GPUSupportRegions) == 0 {
		t.Fatal("GPU support regions should not be empty")
	}

	for _, region := range metadata.GPUSupportRegions {
		if region == nil {
			t.Fatal("Unexpected nil region")
		}

		if region.Name == "" {
			t.Fatal("Region name should not be empty")
		}

		if region.SKUs == nil || len(region.SKUs) == 0 {
			t.Fatalf("Region '%s' SKUs is empty", region.Name)
		}
	}
}
