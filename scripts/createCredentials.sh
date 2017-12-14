#!/bin/bash
# This will build the credentials during the CI
cat <<EOF > ${outputPathCredsfile}
{
  "clientId": "$clientId",
  "clientSecret": "$clientSecret",
  "subscriptionId": "$subscriptionId",
  "tenantId": "$tenantId",
  "activeDirectoryEndpointUrl": "$activeDirectoryEndpointUrl",
  "resourceManagerEndpointUrl": "$resourceManagerEndpointUrl",
  "activeDirectoryGraphResourceId": "$activeDirectoryGraphResourceId",
  "sqlManagementEndpointUrl": "$sqlManagementEndpointUrl",
  "galleryEndpointUrl": "$galleryEndpointUrl",
  "managementEndpointUrl": "$managementEndpointUrl"
}
EOF

# This will build the kubeConfig during the CI
cat <<EOF > ${outputPathKubeConfigFile}
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "$kubeConfigCertificateAuthorityData"
    server: $kubeConfigServer
  name: "aci-connector-k8s"
contexts:
- context:
    cluster: "aci-connector-k8s"
    user: "aci-connector-k8s-admin"
  name: "aci-connector-k8s"
current-context: "aci-connector-k8s"
kind: Config
users:
- name: "aci-connector-k8s-admin"
  user:
    client-certificate-data: "$kubeConfigClientCertificateData"
    client-key-data: "$kubeConfigClientKeyData"
EOF