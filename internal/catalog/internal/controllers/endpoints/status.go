// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package endpoints

const (
	StatusKey                       = "consul.io/endpoint-manager"
	StatusConditionEndpointsManaged = "EndpointsManaged"
	StatusConditionAccepted         = "Accepted"

	StatusReasonSelectorNotFound = "SelectorNotFound"
	StatusReasonSelectorFound    = "SelectorFound"
	StatusReasonPassedValidation = "PassedValidation"

	PassedValidationMessage = "The service has passed validation."
	SelectorFoundMessage    = "A valid workload selector is present within the service."
	SelectorNotFoundMessage = "Either the workload selector was not present or contained no selection criteria."
)
