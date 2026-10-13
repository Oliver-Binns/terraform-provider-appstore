// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oliver-binns/appstore-go/networking"
)

type testResource struct{ ID string }

func shortenRetry(t *testing.T, timeout time.Duration) {
	t.Helper()
	origTimeout, origPoll := apiLookupRetryTimeout, apiLookupRetryPollInterval
	t.Cleanup(func() { apiLookupRetryTimeout, apiLookupRetryPollInterval = origTimeout, origPoll })
	apiLookupRetryTimeout = timeout
	apiLookupRetryPollInterval = 10 * time.Millisecond
}

func TestWaitForResource_RetriesUntilFound(t *testing.T) {
	for name, notFound := range map[string]func() (*testResource, error){
		"error": func() (*testResource, error) { return nil, networking.ErrNotFound },
		"nil":   func() (*testResource, error) { return nil, nil },
	} {
		t.Run(name, func(t *testing.T) {
			shortenRetry(t, time.Second)

			calls := 0
			got, err := waitForResource(context.Background(), func() (*testResource, error) {
				calls++
				if calls < 3 {
					return notFound()
				}
				return &testResource{ID: "found-id"}, nil
			})

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if calls != 3 {
				t.Fatalf("expected 3 calls, got %d", calls)
			}
			if got == nil || got.ID != "found-id" {
				t.Fatalf("expected resource found-id, got %+v", got)
			}
		})
	}
}

func TestWaitForResource_ReturnsErrNotFound_WhenRetriesExhausted(t *testing.T) {
	shortenRetry(t, 20*time.Millisecond)

	got, err := waitForResource(context.Background(), func() (*testResource, error) {
		return nil, networking.ErrNotFound
	})

	if !errors.Is(err, networking.ErrNotFound) {
		t.Fatalf("expected networking.ErrNotFound, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil resource, got %+v", got)
	}
}

func TestWaitForResource_ReturnsNilNil_WhenNilResultNeverFound(t *testing.T) {
	shortenRetry(t, 20*time.Millisecond)

	got, err := waitForResource(context.Background(), func() (*testResource, error) {
		return nil, nil
	})

	if err != nil || got != nil {
		t.Fatalf("expected nil, nil; got %+v, %v", got, err)
	}
}

func TestWaitForResource_ReturnsNonNotFoundErrorImmediately(t *testing.T) {
	shortenRetry(t, time.Minute) // would hang if not returned immediately

	calls := 0
	wantErr := errors.New("API unavailable")
	_, err := waitForResource(context.Background(), func() (*testResource, error) {
		calls++
		return nil, wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", calls)
	}
}
