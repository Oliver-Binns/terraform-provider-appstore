package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/oliver-binns/appstore-go/networking"
)

// Shared across all resources; vars so tests can shorten them.
var apiLookupRetryTimeout = 30 * time.Second
var apiLookupRetryPollInterval = 2 * time.Second

const (
	resourceFoundState    = "found"
	resourceNotFoundState = "not-found"
)

// classifyResourceLookup maps lookup to a retry.StateChangeConf Refresh
// func, treating a networking.ErrNotFound error or a nil result as not-found.
func classifyResourceLookup[T any](lookup func() (*T, error), lastErr *error) func() (interface{}, string, error) {
	return func() (interface{}, string, error) {
		resource, err := lookup()
		if err != nil {
			if errors.Is(err, networking.ErrNotFound) {
				*lastErr = err
				return struct{}{}, resourceNotFoundState, nil
			}
			return nil, "", err
		}
		if resource == nil {
			*lastErr = nil
			return struct{}{}, resourceNotFoundState, nil
		}
		return resource, resourceFoundState, nil
	}
}

// waitForResource polls lookup until found, replaying its not-found signal on timeout.
func waitForResource[T any](ctx context.Context, lookup func() (*T, error)) (*T, error) {
	var lastErr error
	stateConf := &retry.StateChangeConf{
		Pending:    []string{resourceNotFoundState},
		Target:     []string{resourceFoundState},
		Refresh:    classifyResourceLookup(lookup, &lastErr),
		Timeout:    apiLookupRetryTimeout,
		MinTimeout: apiLookupRetryPollInterval,
	}

	result, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		var timeoutErr *retry.TimeoutError
		if errors.As(err, &timeoutErr) {
			return nil, lastErr
		}
		return nil, err
	}
	resourceResult, ok := result.(*T)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T returned while waiting for resource", result)
	}
	return resourceResult, nil
}

// waitForResourceDeleted polls lookup until it reports not-found.
func waitForResourceDeleted[T any](ctx context.Context, lookup func() (*T, error)) error {
	var lastErr error
	stateConf := &retry.StateChangeConf{
		Pending:    []string{resourceFoundState},
		Target:     []string{resourceNotFoundState},
		Refresh:    classifyResourceLookup(lookup, &lastErr),
		Timeout:    apiLookupRetryTimeout,
		MinTimeout: apiLookupRetryPollInterval,
	}

	_, err := stateConf.WaitForStateContext(ctx)
	return err
}
