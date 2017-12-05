#!/bin/bash
# Retrieve kubeconfig for CI
az login --service-principal -u http://azure-cli-2017-11-28-20-22-27 -p $clientSecret --tenant $tenantId
az keyvault secret download --vault-name aciconnectorkv --name kubeconfig -f $PWD/config