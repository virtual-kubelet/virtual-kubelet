package aci

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestLogAnalyticsFileParsingSuccess(t *testing.T) {
	diagnostics, err := NewContainerGroupDiagnosticsFromFile("../../../../loganalytics.json")
	if err != nil {
		t.Fatal(err)
	}

	if diagnostics == nil || diagnostics.LogAnalytics == nil {
		t.Fatalf("Unexpected nil diagnostics. Log Analytics file not parsed correctly")
	}

	if diagnostics.LogAnalytics.WorkspaceID == "" || diagnostics.LogAnalytics.WorkspaceKey == "" {
		t.Fatalf("Unexpected empty analytics authentication credentials. Log Analytics file not parsed correctly")
	}
}

func TestLogAnalyticsFileParsingFailure(t *testing.T) {
	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewContainerGroupDiagnosticsFromFile(tempFile.Name())

	// Cleaup
	tempFile.Close()
	os.Remove(tempFile.Name())

	if err == nil {
		t.Fatalf("Expected parsing an empty Log Analytics auth file to fail, but there were no errors")
	}
}
