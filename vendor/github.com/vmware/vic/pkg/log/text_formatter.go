// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package log

import "github.com/Sirupsen/logrus"

// level strings padded to match the length of the longest level,
// which is "UNKNOWN" currently. Indexed according to levels in
// logrus, e.g. levelStrs[logrus.InfoLevel] == "INFO ".
var levelStrs = []string{
	"PANIC",
	"FATAL",
	"ERROR",
	"WARN ",
	"INFO ",
	"DEBUG",
}

const unknownLevel = "UNKWN"

type TextFormatter struct {
	// TimestampFormat is the format used to print the timestamp.  By default
	// an RFC3339 timestamp is used.
	TimestampFormat string
}

// NewTextFormatter returns a text formatter
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{
		TimestampFormat: "Jan _2 2006 15:04:05.000Z07:00",
	}
}

func levelToString(level logrus.Level) string {
	if level <= logrus.DebugLevel {
		return levelStrs[level]
	}

	return unknownLevel
}

func (f *TextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	t := f.timeStamp(entry)
	l := levelToString(entry.Level)

	return []byte(t + " " + l + " " + entry.Message + "\n"), nil
}

func (f *TextFormatter) timeStamp(entry *logrus.Entry) string {
	return entry.Time.Format(f.TimestampFormat)
}
