package fargate

import (
	"strings"
)

// Regions is the set of AWS regions where a service is available.
// https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/
type Regions []string

var (
	// FargateRegions are AWS regions where Fargate is available.
	FargateRegions = Regions{
		"us-east-1",
	}
)

// Include returns whether the region set includes the given region.
func (r Regions) Include(region string) bool {
	region = strings.ToLower(region)
	region = strings.Trim(region, " ")

	for _, name := range r {
		if name == region {
			return true
		}
	}

	return false
}

// Names returns an array of region names.
func (r Regions) Names() []string {
	names := make([]string, 0, len(r))

	for _, name := range r {
		names = append(names, name)
	}

	return names
}
