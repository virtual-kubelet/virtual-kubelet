package account

// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Code generated by Microsoft (R) AutoRest Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"context"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/validation"
	"net/http"
)

// AccountsClient is the creates an Azure Data Lake Store account management client.
type AccountsClient struct {
	BaseClient
}

// NewAccountsClient creates an instance of the AccountsClient client.
func NewAccountsClient(subscriptionID string) AccountsClient {
	return NewAccountsClientWithBaseURI(DefaultBaseURI, subscriptionID)
}

// NewAccountsClientWithBaseURI creates an instance of the AccountsClient client.
func NewAccountsClientWithBaseURI(baseURI string, subscriptionID string) AccountsClient {
	return AccountsClient{NewWithBaseURI(baseURI, subscriptionID)}
}

// CheckNameAvailability checks whether the specified account name is available or taken.
//
// location is the resource location without whitespace. parameters is parameters supplied to check the Data Lake
// Store account name availability.
func (client AccountsClient) CheckNameAvailability(ctx context.Context, location string, parameters CheckNameAvailabilityParameters) (result NameAvailabilityInformation, err error) {
	if err := validation.Validate([]validation.Validation{
		{TargetValue: parameters,
			Constraints: []validation.Constraint{{Target: "parameters.Name", Name: validation.Null, Rule: true, Chain: nil},
				{Target: "parameters.Type", Name: validation.Null, Rule: true, Chain: nil}}}}); err != nil {
		return result, validation.NewError("account.AccountsClient", "CheckNameAvailability", err.Error())
	}

	req, err := client.CheckNameAvailabilityPreparer(ctx, location, parameters)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "CheckNameAvailability", nil, "Failure preparing request")
		return
	}

	resp, err := client.CheckNameAvailabilitySender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "CheckNameAvailability", resp, "Failure sending request")
		return
	}

	result, err = client.CheckNameAvailabilityResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "CheckNameAvailability", resp, "Failure responding to request")
	}

	return
}

// CheckNameAvailabilityPreparer prepares the CheckNameAvailability request.
func (client AccountsClient) CheckNameAvailabilityPreparer(ctx context.Context, location string, parameters CheckNameAvailabilityParameters) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"location":       autorest.Encode("path", location),
		"subscriptionId": autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/json; charset=utf-8"),
		autorest.AsPost(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/providers/Microsoft.DataLakeStore/locations/{location}/checkNameAvailability", pathParameters),
		autorest.WithJSON(parameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// CheckNameAvailabilitySender sends the CheckNameAvailability request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) CheckNameAvailabilitySender(req *http.Request) (*http.Response, error) {
	return autorest.SendWithSender(client, req,
		azure.DoRetryWithRegistration(client.Client))
}

// CheckNameAvailabilityResponder handles the response to the CheckNameAvailability request. The method always
// closes the http.Response Body.
func (client AccountsClient) CheckNameAvailabilityResponder(resp *http.Response) (result NameAvailabilityInformation, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// Create creates the specified Data Lake Store account.
//
// resourceGroupName is the name of the Azure resource group. accountName is the name of the Data Lake Store
// account. parameters is parameters supplied to create the Data Lake Store account.
func (client AccountsClient) Create(ctx context.Context, resourceGroupName string, accountName string, parameters CreateDataLakeStoreAccountParameters) (result AccountsCreateFutureType, err error) {
	if err := validation.Validate([]validation.Validation{
		{TargetValue: parameters,
			Constraints: []validation.Constraint{{Target: "parameters.Location", Name: validation.Null, Rule: true, Chain: nil},
				{Target: "parameters.Identity", Name: validation.Null, Rule: false,
					Chain: []validation.Constraint{{Target: "parameters.Identity.Type", Name: validation.Null, Rule: true, Chain: nil}}},
				{Target: "parameters.CreateDataLakeStoreAccountProperties", Name: validation.Null, Rule: false,
					Chain: []validation.Constraint{{Target: "parameters.CreateDataLakeStoreAccountProperties.EncryptionConfig", Name: validation.Null, Rule: false,
						Chain: []validation.Constraint{{Target: "parameters.CreateDataLakeStoreAccountProperties.EncryptionConfig.KeyVaultMetaInfo", Name: validation.Null, Rule: false,
							Chain: []validation.Constraint{{Target: "parameters.CreateDataLakeStoreAccountProperties.EncryptionConfig.KeyVaultMetaInfo.KeyVaultResourceID", Name: validation.Null, Rule: true, Chain: nil},
								{Target: "parameters.CreateDataLakeStoreAccountProperties.EncryptionConfig.KeyVaultMetaInfo.EncryptionKeyName", Name: validation.Null, Rule: true, Chain: nil},
								{Target: "parameters.CreateDataLakeStoreAccountProperties.EncryptionConfig.KeyVaultMetaInfo.EncryptionKeyVersion", Name: validation.Null, Rule: true, Chain: nil},
							}},
						}},
					}}}}}); err != nil {
		return result, validation.NewError("account.AccountsClient", "Create", err.Error())
	}

	req, err := client.CreatePreparer(ctx, resourceGroupName, accountName, parameters)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Create", nil, "Failure preparing request")
		return
	}

	result, err = client.CreateSender(req)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Create", result.Response(), "Failure sending request")
		return
	}

	return
}

// CreatePreparer prepares the Create request.
func (client AccountsClient) CreatePreparer(ctx context.Context, resourceGroupName string, accountName string, parameters CreateDataLakeStoreAccountParameters) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"accountName":       autorest.Encode("path", accountName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/json; charset=utf-8"),
		autorest.AsPut(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.DataLakeStore/accounts/{accountName}", pathParameters),
		autorest.WithJSON(parameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// CreateSender sends the Create request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) CreateSender(req *http.Request) (future AccountsCreateFutureType, err error) {
	sender := autorest.DecorateSender(client, azure.DoRetryWithRegistration(client.Client))
	future.Future = azure.NewFuture(req)
	future.req = req
	_, err = future.Done(sender)
	if err != nil {
		return
	}
	err = autorest.Respond(future.Response(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated))
	return
}

// CreateResponder handles the response to the Create request. The method always
// closes the http.Response Body.
func (client AccountsClient) CreateResponder(resp *http.Response) (result DataLakeStoreAccount, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// Delete deletes the specified Data Lake Store account.
//
// resourceGroupName is the name of the Azure resource group. accountName is the name of the Data Lake Store
// account.
func (client AccountsClient) Delete(ctx context.Context, resourceGroupName string, accountName string) (result AccountsDeleteFutureType, err error) {
	req, err := client.DeletePreparer(ctx, resourceGroupName, accountName)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Delete", nil, "Failure preparing request")
		return
	}

	result, err = client.DeleteSender(req)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Delete", result.Response(), "Failure sending request")
		return
	}

	return
}

// DeletePreparer prepares the Delete request.
func (client AccountsClient) DeletePreparer(ctx context.Context, resourceGroupName string, accountName string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"accountName":       autorest.Encode("path", accountName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsDelete(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.DataLakeStore/accounts/{accountName}", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// DeleteSender sends the Delete request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) DeleteSender(req *http.Request) (future AccountsDeleteFutureType, err error) {
	sender := autorest.DecorateSender(client, azure.DoRetryWithRegistration(client.Client))
	future.Future = azure.NewFuture(req)
	future.req = req
	_, err = future.Done(sender)
	if err != nil {
		return
	}
	err = autorest.Respond(future.Response(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted, http.StatusNoContent))
	return
}

// DeleteResponder handles the response to the Delete request. The method always
// closes the http.Response Body.
func (client AccountsClient) DeleteResponder(resp *http.Response) (result autorest.Response, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted, http.StatusNoContent),
		autorest.ByClosing())
	result.Response = resp
	return
}

// EnableKeyVault attempts to enable a user managed Key Vault for encryption of the specified Data Lake Store account.
//
// resourceGroupName is the name of the Azure resource group. accountName is the name of the Data Lake Store
// account.
func (client AccountsClient) EnableKeyVault(ctx context.Context, resourceGroupName string, accountName string) (result autorest.Response, err error) {
	req, err := client.EnableKeyVaultPreparer(ctx, resourceGroupName, accountName)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "EnableKeyVault", nil, "Failure preparing request")
		return
	}

	resp, err := client.EnableKeyVaultSender(req)
	if err != nil {
		result.Response = resp
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "EnableKeyVault", resp, "Failure sending request")
		return
	}

	result, err = client.EnableKeyVaultResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "EnableKeyVault", resp, "Failure responding to request")
	}

	return
}

// EnableKeyVaultPreparer prepares the EnableKeyVault request.
func (client AccountsClient) EnableKeyVaultPreparer(ctx context.Context, resourceGroupName string, accountName string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"accountName":       autorest.Encode("path", accountName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsPost(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.DataLakeStore/accounts/{accountName}/enableKeyVault", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// EnableKeyVaultSender sends the EnableKeyVault request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) EnableKeyVaultSender(req *http.Request) (*http.Response, error) {
	return autorest.SendWithSender(client, req,
		azure.DoRetryWithRegistration(client.Client))
}

// EnableKeyVaultResponder handles the response to the EnableKeyVault request. The method always
// closes the http.Response Body.
func (client AccountsClient) EnableKeyVaultResponder(resp *http.Response) (result autorest.Response, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByClosing())
	result.Response = resp
	return
}

// Get gets the specified Data Lake Store account.
//
// resourceGroupName is the name of the Azure resource group. accountName is the name of the Data Lake Store
// account.
func (client AccountsClient) Get(ctx context.Context, resourceGroupName string, accountName string) (result DataLakeStoreAccount, err error) {
	req, err := client.GetPreparer(ctx, resourceGroupName, accountName)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Get", nil, "Failure preparing request")
		return
	}

	resp, err := client.GetSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Get", resp, "Failure sending request")
		return
	}

	result, err = client.GetResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Get", resp, "Failure responding to request")
	}

	return
}

// GetPreparer prepares the Get request.
func (client AccountsClient) GetPreparer(ctx context.Context, resourceGroupName string, accountName string) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"accountName":       autorest.Encode("path", accountName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.DataLakeStore/accounts/{accountName}", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// GetSender sends the Get request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) GetSender(req *http.Request) (*http.Response, error) {
	return autorest.SendWithSender(client, req,
		azure.DoRetryWithRegistration(client.Client))
}

// GetResponder handles the response to the Get request. The method always
// closes the http.Response Body.
func (client AccountsClient) GetResponder(resp *http.Response) (result DataLakeStoreAccount, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// List lists the Data Lake Store accounts within the subscription. The response includes a link to the next page of
// results, if any.
//
// filter is oData filter. Optional. top is the number of items to return. Optional. skip is the number of items to
// skip over before returning elements. Optional. selectParameter is oData Select statement. Limits the properties
// on each entry to just those requested, e.g. Categories?$select=CategoryName,Description. Optional. orderby is
// orderBy clause. One or more comma-separated expressions with an optional "asc" (the default) or "desc" depending
// on the order you'd like the values sorted, e.g. Categories?$orderby=CategoryName desc. Optional. count is the
// Boolean value of true or false to request a count of the matching resources included with the resources in the
// response, e.g. Categories?$count=true. Optional.
func (client AccountsClient) List(ctx context.Context, filter string, top *int32, skip *int32, selectParameter string, orderby string, count *bool) (result DataLakeStoreAccountListResultPage, err error) {
	if err := validation.Validate([]validation.Validation{
		{TargetValue: top,
			Constraints: []validation.Constraint{{Target: "top", Name: validation.Null, Rule: false,
				Chain: []validation.Constraint{{Target: "top", Name: validation.InclusiveMinimum, Rule: 1, Chain: nil}}}}},
		{TargetValue: skip,
			Constraints: []validation.Constraint{{Target: "skip", Name: validation.Null, Rule: false,
				Chain: []validation.Constraint{{Target: "skip", Name: validation.InclusiveMinimum, Rule: 1, Chain: nil}}}}}}); err != nil {
		return result, validation.NewError("account.AccountsClient", "List", err.Error())
	}

	result.fn = client.listNextResults
	req, err := client.ListPreparer(ctx, filter, top, skip, selectParameter, orderby, count)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "List", nil, "Failure preparing request")
		return
	}

	resp, err := client.ListSender(req)
	if err != nil {
		result.dlsalr.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "List", resp, "Failure sending request")
		return
	}

	result.dlsalr, err = client.ListResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "List", resp, "Failure responding to request")
	}

	return
}

// ListPreparer prepares the List request.
func (client AccountsClient) ListPreparer(ctx context.Context, filter string, top *int32, skip *int32, selectParameter string, orderby string, count *bool) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"subscriptionId": autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}
	if len(filter) > 0 {
		queryParameters["$filter"] = autorest.Encode("query", filter)
	}
	if top != nil {
		queryParameters["$top"] = autorest.Encode("query", *top)
	}
	if skip != nil {
		queryParameters["$skip"] = autorest.Encode("query", *skip)
	}
	if len(selectParameter) > 0 {
		queryParameters["$select"] = autorest.Encode("query", selectParameter)
	}
	if len(orderby) > 0 {
		queryParameters["$orderby"] = autorest.Encode("query", orderby)
	}
	if count != nil {
		queryParameters["$count"] = autorest.Encode("query", *count)
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/providers/Microsoft.DataLakeStore/accounts", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// ListSender sends the List request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) ListSender(req *http.Request) (*http.Response, error) {
	return autorest.SendWithSender(client, req,
		azure.DoRetryWithRegistration(client.Client))
}

// ListResponder handles the response to the List request. The method always
// closes the http.Response Body.
func (client AccountsClient) ListResponder(resp *http.Response) (result DataLakeStoreAccountListResult, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// listNextResults retrieves the next set of results, if any.
func (client AccountsClient) listNextResults(lastResults DataLakeStoreAccountListResult) (result DataLakeStoreAccountListResult, err error) {
	req, err := lastResults.dataLakeStoreAccountListResultPreparer()
	if err != nil {
		return result, autorest.NewErrorWithError(err, "account.AccountsClient", "listNextResults", nil, "Failure preparing next results request")
	}
	if req == nil {
		return
	}
	resp, err := client.ListSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		return result, autorest.NewErrorWithError(err, "account.AccountsClient", "listNextResults", resp, "Failure sending next results request")
	}
	result, err = client.ListResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "listNextResults", resp, "Failure responding to next results request")
	}
	return
}

// ListComplete enumerates all values, automatically crossing page boundaries as required.
func (client AccountsClient) ListComplete(ctx context.Context, filter string, top *int32, skip *int32, selectParameter string, orderby string, count *bool) (result DataLakeStoreAccountListResultIterator, err error) {
	result.page, err = client.List(ctx, filter, top, skip, selectParameter, orderby, count)
	return
}

// ListByResourceGroup lists the Data Lake Store accounts within a specific resource group. The response includes a
// link to the next page of results, if any.
//
// resourceGroupName is the name of the Azure resource group. filter is oData filter. Optional. top is the number
// of items to return. Optional. skip is the number of items to skip over before returning elements. Optional.
// selectParameter is oData Select statement. Limits the properties on each entry to just those requested, e.g.
// Categories?$select=CategoryName,Description. Optional. orderby is orderBy clause. One or more comma-separated
// expressions with an optional "asc" (the default) or "desc" depending on the order you'd like the values sorted,
// e.g. Categories?$orderby=CategoryName desc. Optional. count is a Boolean value of true or false to request a
// count of the matching resources included with the resources in the response, e.g. Categories?$count=true.
// Optional.
func (client AccountsClient) ListByResourceGroup(ctx context.Context, resourceGroupName string, filter string, top *int32, skip *int32, selectParameter string, orderby string, count *bool) (result DataLakeStoreAccountListResultPage, err error) {
	if err := validation.Validate([]validation.Validation{
		{TargetValue: top,
			Constraints: []validation.Constraint{{Target: "top", Name: validation.Null, Rule: false,
				Chain: []validation.Constraint{{Target: "top", Name: validation.InclusiveMinimum, Rule: 1, Chain: nil}}}}},
		{TargetValue: skip,
			Constraints: []validation.Constraint{{Target: "skip", Name: validation.Null, Rule: false,
				Chain: []validation.Constraint{{Target: "skip", Name: validation.InclusiveMinimum, Rule: 1, Chain: nil}}}}}}); err != nil {
		return result, validation.NewError("account.AccountsClient", "ListByResourceGroup", err.Error())
	}

	result.fn = client.listByResourceGroupNextResults
	req, err := client.ListByResourceGroupPreparer(ctx, resourceGroupName, filter, top, skip, selectParameter, orderby, count)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "ListByResourceGroup", nil, "Failure preparing request")
		return
	}

	resp, err := client.ListByResourceGroupSender(req)
	if err != nil {
		result.dlsalr.Response = autorest.Response{Response: resp}
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "ListByResourceGroup", resp, "Failure sending request")
		return
	}

	result.dlsalr, err = client.ListByResourceGroupResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "ListByResourceGroup", resp, "Failure responding to request")
	}

	return
}

// ListByResourceGroupPreparer prepares the ListByResourceGroup request.
func (client AccountsClient) ListByResourceGroupPreparer(ctx context.Context, resourceGroupName string, filter string, top *int32, skip *int32, selectParameter string, orderby string, count *bool) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}
	if len(filter) > 0 {
		queryParameters["$filter"] = autorest.Encode("query", filter)
	}
	if top != nil {
		queryParameters["$top"] = autorest.Encode("query", *top)
	}
	if skip != nil {
		queryParameters["$skip"] = autorest.Encode("query", *skip)
	}
	if len(selectParameter) > 0 {
		queryParameters["$select"] = autorest.Encode("query", selectParameter)
	}
	if len(orderby) > 0 {
		queryParameters["$orderby"] = autorest.Encode("query", orderby)
	}
	if count != nil {
		queryParameters["$count"] = autorest.Encode("query", *count)
	}

	preparer := autorest.CreatePreparer(
		autorest.AsGet(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.DataLakeStore/accounts", pathParameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// ListByResourceGroupSender sends the ListByResourceGroup request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) ListByResourceGroupSender(req *http.Request) (*http.Response, error) {
	return autorest.SendWithSender(client, req,
		azure.DoRetryWithRegistration(client.Client))
}

// ListByResourceGroupResponder handles the response to the ListByResourceGroup request. The method always
// closes the http.Response Body.
func (client AccountsClient) ListByResourceGroupResponder(resp *http.Response) (result DataLakeStoreAccountListResult, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}

// listByResourceGroupNextResults retrieves the next set of results, if any.
func (client AccountsClient) listByResourceGroupNextResults(lastResults DataLakeStoreAccountListResult) (result DataLakeStoreAccountListResult, err error) {
	req, err := lastResults.dataLakeStoreAccountListResultPreparer()
	if err != nil {
		return result, autorest.NewErrorWithError(err, "account.AccountsClient", "listByResourceGroupNextResults", nil, "Failure preparing next results request")
	}
	if req == nil {
		return
	}
	resp, err := client.ListByResourceGroupSender(req)
	if err != nil {
		result.Response = autorest.Response{Response: resp}
		return result, autorest.NewErrorWithError(err, "account.AccountsClient", "listByResourceGroupNextResults", resp, "Failure sending next results request")
	}
	result, err = client.ListByResourceGroupResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "listByResourceGroupNextResults", resp, "Failure responding to next results request")
	}
	return
}

// ListByResourceGroupComplete enumerates all values, automatically crossing page boundaries as required.
func (client AccountsClient) ListByResourceGroupComplete(ctx context.Context, resourceGroupName string, filter string, top *int32, skip *int32, selectParameter string, orderby string, count *bool) (result DataLakeStoreAccountListResultIterator, err error) {
	result.page, err = client.ListByResourceGroup(ctx, resourceGroupName, filter, top, skip, selectParameter, orderby, count)
	return
}

// Update updates the specified Data Lake Store account information.
//
// resourceGroupName is the name of the Azure resource group. accountName is the name of the Data Lake Store
// account. parameters is parameters supplied to update the Data Lake Store account.
func (client AccountsClient) Update(ctx context.Context, resourceGroupName string, accountName string, parameters UpdateDataLakeStoreAccountParameters) (result AccountsUpdateFutureType, err error) {
	req, err := client.UpdatePreparer(ctx, resourceGroupName, accountName, parameters)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Update", nil, "Failure preparing request")
		return
	}

	result, err = client.UpdateSender(req)
	if err != nil {
		err = autorest.NewErrorWithError(err, "account.AccountsClient", "Update", result.Response(), "Failure sending request")
		return
	}

	return
}

// UpdatePreparer prepares the Update request.
func (client AccountsClient) UpdatePreparer(ctx context.Context, resourceGroupName string, accountName string, parameters UpdateDataLakeStoreAccountParameters) (*http.Request, error) {
	pathParameters := map[string]interface{}{
		"accountName":       autorest.Encode("path", accountName),
		"resourceGroupName": autorest.Encode("path", resourceGroupName),
		"subscriptionId":    autorest.Encode("path", client.SubscriptionID),
	}

	const APIVersion = "2016-11-01"
	queryParameters := map[string]interface{}{
		"api-version": APIVersion,
	}

	preparer := autorest.CreatePreparer(
		autorest.AsContentType("application/json; charset=utf-8"),
		autorest.AsPatch(),
		autorest.WithBaseURL(client.BaseURI),
		autorest.WithPathParameters("/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.DataLakeStore/accounts/{accountName}", pathParameters),
		autorest.WithJSON(parameters),
		autorest.WithQueryParameters(queryParameters))
	return preparer.Prepare((&http.Request{}).WithContext(ctx))
}

// UpdateSender sends the Update request. The method will close the
// http.Response Body if it receives an error.
func (client AccountsClient) UpdateSender(req *http.Request) (future AccountsUpdateFutureType, err error) {
	sender := autorest.DecorateSender(client, azure.DoRetryWithRegistration(client.Client))
	future.Future = azure.NewFuture(req)
	future.req = req
	_, err = future.Done(sender)
	if err != nil {
		return
	}
	err = autorest.Respond(future.Response(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated, http.StatusAccepted))
	return
}

// UpdateResponder handles the response to the Update request. The method always
// closes the http.Response Body.
func (client AccountsClient) UpdateResponder(resp *http.Response) (result DataLakeStoreAccount, err error) {
	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated, http.StatusAccepted),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return
}
