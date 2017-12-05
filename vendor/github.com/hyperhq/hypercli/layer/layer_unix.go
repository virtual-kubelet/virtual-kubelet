// +build linux freebsd darwin

package layer

import "github.com/hyperhq/hypercli/pkg/stringid"

func (ls *layerStore) mountID(name string) string {
	return stringid.GenerateRandomID()
}
