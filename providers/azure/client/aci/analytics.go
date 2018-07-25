package aci

import "errors"

func NewContainerGroupDiagnostics(logAnalyticsID, logAnalyticsKey string) (*ContainerGroupDiagnostics, error) {

	if logAnalyticsID == "" || logAnalyticsKey == "" {
		return nil, errors.New("Log Analytics configuration requires both the workspace ID and Key")
	}

	return &ContainerGroupDiagnostics{
		LogAnalytics: &LogAnalyticsWorkspace{
			WorkspaceID:  logAnalyticsID,
			WorkspaceKey: logAnalyticsKey,
		},
	}, nil
}
