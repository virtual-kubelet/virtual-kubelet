// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package converter

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/pkg/ip"
	viclog "github.com/vmware/vic/pkg/log"
)

const (
	cmdTag         = "cmd"
	labelTag       = "label"
	parentTagValue = "parent"

	optionSeparator = "-"

	keyAfterValueLabel = "value-key"
	valueAfterKeyLabel = "key-value"
)

var (
	kindConverters = make(map[reflect.Kind]converter)
	typeConverters = make(map[string]converter)
	labelHandlers  = make(map[string]labelConverter)
)

var log = &logrus.Logger{
	Out: os.Stderr,
	// We're using our own text formatter to skip the \n and \t escaping logrus
	// was doing on non TTY Out (we redirect to a file) descriptors.
	Formatter: viclog.NewTextFormatter(),
	Hooks:     make(logrus.LevelHooks),
	Level:     logrus.InfoLevel,
}

type converter func(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error
type labelConverter func(dest map[string][]string, key string) error

func init() {
	kindConverters[reflect.Struct] = convertStruct
	kindConverters[reflect.Slice] = convertSlice
	kindConverters[reflect.Map] = convertMap
	kindConverters[reflect.String] = convertString
	kindConverters[reflect.Ptr] = convertPtr
	kindConverters[reflect.Int] = convertPrimitive
	kindConverters[reflect.Int8] = convertPrimitive
	kindConverters[reflect.Int16] = convertPrimitive
	kindConverters[reflect.Int32] = convertPrimitive
	kindConverters[reflect.Int64] = convertPrimitive
	kindConverters[reflect.Bool] = convertPrimitive
	kindConverters[reflect.Float32] = convertPrimitive
	kindConverters[reflect.Float64] = convertPrimitive

	typeConverters["url.URL"] = convertURL
	typeConverters["net.IPNet"] = convertIPNet
	typeConverters["net.IP"] = convertIP
	typeConverters["ip.Range"] = convertIPRange
	typeConverters["data.NetworkConfig"] = convertNetwork
	typeConverters["common.ContainerNetworks"] = convertContainerNetworks
	typeConverters["executor.TrustLevel"] = convertTrustLevel
	typeConverters["types.SharesInfo"] = convertShares

	labelHandlers[keyAfterValueLabel] = keyAfterValueLabelHandler
	labelHandlers[valueAfterKeyLabel] = valueAfterKeyLabelHandler
}

func EnableLog() {
	log.Level = logrus.DebugLevel
}

func DisableLog() {
	log.Level = logrus.InfoLevel
}

// DataToOption convert data.Data structure to vic-machine create command options based on tags defined in data.Data structure
// Note: need to make sure the tags are consistent with command line option name
func DataToOption(data *data.Data) (map[string][]string, error) {
	result := make(map[string][]string)

	if data == nil {
		return result, nil
	}
	err := convert(reflect.ValueOf(data), "", "", result)
	return result, err
}

func convert(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	t := src.Type().String()
	if converter, ok := typeConverters[t]; ok {
		return converter(src, prefix, tags, dest)

	}
	if converter, ok := kindConverters[src.Kind()]; ok {
		return converter(src, prefix, tags, dest)
	}
	log.Debugf("Skipping unsupported field, interface: %#v, kind %s", src, src.Kind())
	return nil
}

func convertPtr(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	if src.IsNil() {
		// no need to attempt anything
		return nil
	}
	return convert(src.Elem(), prefix, tags, dest)
}

func convertStruct(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertStruct: prefix: %s, src: %s", prefix, src.String())

	// iterate through every field in the struct
	for i := 0; i < src.NumField(); i++ {
		field := src.Field(i)
		// get field key, and keep going even if the attribute key is empty, to make sure children attribute is not missing
		tags := src.Type().Field(i).Tag
		key := calculateKey(tags, prefix)
		if err := convert(field, key, tags, dest); err != nil {
			return err
		}

		if field.Kind() == reflect.Map {
			// label handler is invoked in map converter
			continue
		}
	}
	return nil
}

func convertSlice(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertSlice: prefix: %s, src: %s", prefix, src)

	length := src.Len()
	if length == 0 {
		log.Debug("Skipping empty slice")
		return nil
	}

	for i := 0; i < length; i++ {
		if err := convert(src.Index(i), prefix, tags, dest); err != nil {
			return err
		}
	}
	return nil
}

func convertMap(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertMap: prefix: %s, src: %s", prefix, src)

	// iterate over keys and recurse
	mkeys := src.MapKeys()
	length := len(mkeys)
	if length == 0 {
		log.Debug("Skipping empty map")
		return nil
	}

	handler, hasHandler := labelHandlers[tags.Get(labelTag)]
	// use tempMap to avoid duplicate processing
	for _, pkey := range src.MapKeys() {
		tempMap := make(map[string][]string)
		if pkey.Kind() != reflect.String {
			log.Errorf("Unsupported map key type interface: %s, kind %s", src, src.Kind())
			continue
		}
		if !hasHandler {
			if err := convert(src.MapIndex(pkey), prefix, tags, dest); err != nil {
				return err
			}
			continue
		}
		if err := convert(src.MapIndex(pkey), prefix, tags, tempMap); err != nil {
			return err
		}
		if err := handler(tempMap, pkey.String()); err != nil {
			return err
		}
		for k, v := range tempMap {
			addValues(dest, k, v)
		}
	}
	return nil
}

// keyAfterValueLabelHandler will add the map key as label after the value,
// e.g. change from datastore/path to datastore/path:default
func keyAfterValueLabelHandler(dest map[string][]string, pkey string) error {
	log.Debugf("keyAfterValueLabelHandler: map key: %s, map: %#v", pkey, dest)

	for _, values := range dest {
		for i := range values {
			values[i] = fmt.Sprintf("%s:%s", values[i], pkey)
		}
	}
	return nil
}

// valueAfterKeyLabelHandler will add the map key as label before the value,
// e.g. change from 10.10.10.0/24 to management:10.10.10.0/24
func valueAfterKeyLabelHandler(dest map[string][]string, pkey string) error {
	log.Debugf("valueAfterKeyLabelHandler: map key: %s, map: %#v", pkey, dest)

	for _, values := range dest {
		for i := range values {
			values[i] = fmt.Sprintf("%s:%s", pkey, values[i])
		}
	}
	return nil
}

// calculateKey generate key as prefix-tag. if any one is empty, return the other
func calculateKey(tags reflect.StructTag, prefix string) string {
	tag := tags.Get(cmdTag)
	if tag == "" {
		return prefix
	}
	if tag == parentTagValue && prefix == "" {
		return ""
	}
	if tag == parentTagValue && prefix != "" {
		// for this tag, use parent name only
		return prefix
	}
	if prefix == "" {
		return tag
	}
	return fmt.Sprintf("%s%s%s", prefix, optionSeparator, tag)
}

func convertURL(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertURL: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	u, ok := src.Interface().(url.URL)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not URL"))
	}
	v := u.String()
	if u.Scheme == "" {
		if u.Path == "" {
			v = u.Host
		} else if u.Host == "" {
			v = u.Path
		} else {
			v = fmt.Sprintf("%s/%s", u.Host, u.Path)
		}
	}

	log.Debugf("%s=%s", prefix, v)
	addValue(dest, prefix, v)
	return nil
}

func convertIPNet(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertIPNet: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	ipNet, ok := src.Interface().(net.IPNet)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not IPNet"))
	}
	if ip.IsUnspecifiedSubnet(&ipNet) {
		return nil
	}
	v := ipNet.String()

	log.Debugf("%s=%s", prefix, v)
	addValue(dest, prefix, v)
	return nil
}

func convertIP(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertIP: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	ipAddr, ok := src.Interface().(net.IP)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not IP"))
	}
	if ip.IsUnspecifiedIP(ipAddr) {
		return nil
	}
	v := ipAddr.String()

	log.Debugf("%s=%s", prefix, v)
	addValue(dest, prefix, v)
	return nil
}

func convertTrustLevel(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertTrustLevel: prefix: %s, src: %s", prefix, src.String())
	trustLevel, ok := src.Interface().(executor.TrustLevel)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not TrustLevel"))
	}
	if trustLevel == executor.Unspecified {
		return nil
	}
	v := trustLevel.String()

	log.Debugf("%s=%s", prefix, v)
	addValue(dest, prefix, v)
	return nil
}

func convertIPRange(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertIPRange: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	ipRange, ok := src.Interface().(ip.Range)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not ip range"))
	}
	v := ipRange.String()
	if v == "" {
		return nil
	}

	log.Debugf("%s=%s", prefix, v)
	addValue(dest, prefix, v)
	return nil
}

func convertString(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertString: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	v := src.String()
	if v == "" {
		return nil
	}

	log.Debugf("%s=%s", prefix, v)

	addValue(dest, prefix, v)
	return nil
}

func convertPrimitive(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertPrimitive: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	v := ""
	switch src.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if src.Int() == 0 {
			return nil
		}
		v = strconv.FormatInt(src.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if src.Uint() == 0 {
			return nil
		}
		v = strconv.FormatUint(src.Uint(), 10)
	case reflect.Bool:
		if !src.Bool() {
			return nil
		}
		v = strconv.FormatBool(src.Bool())
	case reflect.Float32, reflect.Float64:
		if src.Float() == 0 {
			return nil
		}
		v = strconv.FormatFloat(src.Float(), 'E', -1, 64)
	default:
		panic(fmt.Sprintf("%s is not supported type", src.Kind()))
	}
	log.Debugf("%s=%s", prefix, v)

	addValue(dest, prefix, v)
	return nil
}

// convertNetwork will merge destination and gateway to one option with format: 192.168.3.0/16,192.168.128.0/16:192.168.2.1
// after that, convertStruct is called for left conversion
func convertNetwork(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertNetwork: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	network, ok := src.Interface().(data.NetworkConfig)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not NetworkConfig"))
	}
	if !network.IsSet() {
		return nil
	}

	if len(network.Destinations) > 0 || !ip.Empty(network.Gateway) {
		destination := ""
		if len(network.Destinations) > 0 {
			for _, d := range network.Destinations {
				destination = fmt.Sprintf("%s,%s", destination, d.String())
			}
			destination = strings.TrimLeft(destination, ",")
		}
		gateway := network.Gateway.IP.String()
		tag := "cmd:\"gateway\""
		key := calculateKey(reflect.StructTag(tag), prefix)
		if destination != "" {
			addValue(dest, key, fmt.Sprintf("%s:%s", destination, gateway))
		} else {
			addValue(dest, key, gateway)
		}
	}
	return convertStruct(reflect.ValueOf(network), prefix, tags, dest)
}

func convertShares(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertShares: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}
	shares, ok := src.Interface().(types.SharesInfo)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not SharesInfo"))
	}

	v := ""
	switch shares.Level {
	case types.SharesLevelCustom:
		v = fmt.Sprintf("%v", shares.Shares)
	case types.SharesLevelNormal:
		return nil
	default:
		v = string(shares.Level)
	}

	log.Debugf("%s=%s", prefix, v)

	addValue(dest, prefix, v)
	return nil
}

// convertContainerNetworks will switch the map keys in MappedNetworks using value, and replace all keys with the same value in other structure,
// cause option is using vsphere network name as key label, but guestinfo is using alias as key for easy to use in portlayer
// after that, convertStruct is called for left conversion
func convertContainerNetworks(src reflect.Value, prefix string, tags reflect.StructTag, dest map[string][]string) error {
	log.Debugf("convertContainerNetworks: prefix: %s, src: %s", prefix, src.String())

	if prefix == "" {
		return nil
	}
	if tags.Get(cmdTag) == "" {
		return nil
	}

	networks, ok := src.Interface().(common.ContainerNetworks)
	if !ok {
		panic(fmt.Sprintf(src.Type().String() + " is not ContainerNetworks"))
	}
	if !networks.IsSet() {
		return nil
	}

	newMappedNetworks := make(map[string]string, len(networks.MappedNetworks))
	for k, v := range networks.MappedNetworks {
		if k == v {
			newMappedNetworks[v] = k
			continue
		}
		if dns, ok := networks.MappedNetworksDNS[k]; ok {
			networks.MappedNetworksDNS[v] = dns
			delete(networks.MappedNetworksDNS, k)
		}
		if gateways, ok := networks.MappedNetworksGateways[k]; ok {
			networks.MappedNetworksGateways[v] = gateways
			delete(networks.MappedNetworksGateways, k)
		}
		if ipRange, ok := networks.MappedNetworksIPRanges[k]; ok {
			networks.MappedNetworksIPRanges[v] = ipRange
			delete(networks.MappedNetworksIPRanges, k)
		}
		if firewall, ok := networks.MappedNetworksFirewalls[k]; ok {
			networks.MappedNetworksFirewalls[v] = firewall
			delete(networks.MappedNetworksFirewalls, k)
		}
		newMappedNetworks[v] = k
	}
	networks.MappedNetworks = newMappedNetworks
	return convertStruct(reflect.ValueOf(networks), prefix, tags, dest)
}

// addValue will append value without duplicates
func addValue(dest map[string][]string, key, value string) {
	slice, _ := dest[key]
	found := false
	for _, o := range slice {
		if o == value {
			found = true
			break
		}
	}
	if !found {
		slice = append(slice, value)
	}
	dest[key] = slice
}

// addValues append new value to existing slice if missing
// as this method is called every time the value is appended, the existing slice will be no duplicates
func addValues(dest map[string][]string, key string, values []string) {
	for _, v := range values {
		addValue(dest, key, v)
	}
}
