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

	if metadata.GPURegionalSKUs == nil || len(metadata.GPURegionalSKUs) == 0 {
		t.Fatal("GPU regional SKUs should not be empty")
	}

	for _, region := range metadata.GPURegionalSKUs {
		if region == nil {
			t.Fatal("Unexpected nil region")
		}

		if region.Location == "" {
			t.Fatal("Region name should not be empty")
		}

		if region.SKUs == nil || len(region.SKUs) == 0 {
			t.Fatalf("Region '%s' SKUs is empty", region.Location)
		}
	}
}
