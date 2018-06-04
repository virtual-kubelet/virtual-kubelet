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

// Package i18n provides functionality to retrieve strings from a
// messages catalog based on a key (string ID).
//
// The messages catalog is loaded from a file containing all of the
// strings for a given language. Here is an example:
//
// English
//
//			File: messages/en
//
// 			greeting=hello
// 			thanks=thank you
//
// Spanish
//
// 			File: messages/es
//
// 			greeting=hola
// 			thanks=gracias
//
// To use the translation functionality, call LoadLanguage with the desired
// messages file for the local language. Replace uses of string literals
// with a call to T and a given string ID. If the string ID is not present
// in the loaded messages file, the string ID will be returned as the default.
//
// Once initialized, Printer can also be used directly with fmt-like print functions.
// This can be used to include format strings for localization as below:
//
// 			val := Printer.Sprintf("You know nothing %s", "Jon Snow")
//
// The corresponding messages catalogs would be:
//
// 			File: messages/en
//
// 			You know nothing %s=You know nothing %s
//
// 			File: messages/es
//
// 			You know nothing %s=No sabes nada %s
//
//
// Packaging message catalogs
//
// Instead of loading a messages file from an on disk file, it is possible to
// load it from a byte array that has been included with the source.
//
// For an executable's messages files in the messages directory, use
// https://github.com/jteeuwen/go-bindata to convert these files to Go source code.
// This MUST be done whenever any file in messages is added or edited.
//
// 			go-bindata -o messages.go messages
//
// Use the messages by recovering the byte[] corresponding to the file in the
// messages directory and loading it.
//
//			data, err := Asset("messages/en")
//			i18n.LoadLanguageBytes(language.English, data)
//
package i18n

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Printer is the message printer used by T
// to obtain a translated value.
var Printer *message.Printer

// DefaultLang is the default langugage, set to English
var DefaultLang = language.English

func getPrinter(scanner *bufio.Scanner, lang language.Tag) (*message.Printer, error) {
	catalog := message.DefaultCatalog
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		s := scanner.Text()
		line := strings.SplitN(s, "=", 2)
		if len(line) != 2 {
			return nil, fmt.Errorf("Invalid line in messages file: %v", line)
		}
		catalog.SetString(lang, line[0], line[1])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return catalog.Printer(lang), nil
}

// LoadLanguage sets the package level Printer after loading
// the desired language file from the path messagesFile.
func LoadLanguage(lang language.Tag, messagesFile string) error {
	f, err := os.Open(messagesFile)
	if err != nil {
		return fmt.Errorf("Failed to open messages file: %v", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	return loadLanguageScanner(lang, s)
}

// LoadLanguageBytes sets the package level Printer after loading
// the desired language data from a byte array
func LoadLanguageBytes(lang language.Tag, messagesData []byte) error {
	data := bytes.NewReader(messagesData)
	s := bufio.NewScanner(data)
	return loadLanguageScanner(lang, s)
}

func loadLanguageScanner(lang language.Tag, s *bufio.Scanner) error {
	printer, err := getPrinter(s, lang)
	if err == nil {
		Printer = printer
	}
	return err
}

// T (Translate) takes the message key stringID and returns the string
// value that the key maps to in the loaded messages file. If the key
// is not found, stringID is returned as the default value.
func T(stringID string) string {
	k := message.Key(stringID, stringID)
	if Printer == nil {
		panic("Message file has not been loaded. Call LoadLanguage.")
	}
	return Printer.Sprintf(k)
}
