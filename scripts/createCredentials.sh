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