// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package i18n

import (
	"bufio"
	"strings"
	"testing"

	"golang.org/x/text/language"
)

const testData = `key1=message1 english
key2=message2 english
format key %s=format message english %s`

const badTestData = `key1message1`

func TestGetPrinter(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(testData))
	p, err := getPrinter(scanner, language.English)
	if p == nil {
		t.Errorf("Failed to getPrinter: %s", err)
	}
}

func TestGetPrinterInvalid(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(badTestData))
	p, err := getPrinter(scanner, language.English)
	if err == nil {
		t.Errorf("Failed to get an expected error.")
	}
	if p != nil {
		t.Errorf("Got an unexpected printer from getPrinter")
	}
}

func TestPrinterFormat(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(testData))
	p, _ := getPrinter(scanner, language.English)
	Printer = p
	testKey := "format key %s"
	expectedValue := "format message english HAI"

	val := Printer.Sprintf(testKey, "HAI")
	if val != expectedValue {
		t.Errorf("Got: %s Expected: %s", val, expectedValue)
	}
}

func TestTranslate(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(testData))
	p, _ := getPrinter(scanner, language.English)
	Printer = p
	testKey := "key1"
	expectedValue := "message1 english"

	val := T(testKey)
	if val != expectedValue {
		t.Errorf("Got: %s Expected: %s", val, expectedValue)
	}
}

func TestTranslateDefault(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(testData))
	p, _ := getPrinter(scanner, language.English)
	Printer = p
	testKey := "undefined"

	val := T(testKey)
	if val != testKey {
		t.Errorf("Got: %s Expected: %s", val, testKey)
	}
}

func TestUnloadedTranslate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Did not receive expected panic.")
		}
	}()

	Printer = nil
	testKey := "key1"
	T(testKey)
}

func TestLoadLanguageScanner(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader(testData))
	err := loadLanguageScanner(language.English, scanner)
	if err != nil {
		t.Errorf("Failed to getPrinter")
	}
	if Printer == nil {
		t.Errorf("Failed to set Printer")
	}
}
