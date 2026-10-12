// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oliver-binns/appstore-go/users"
)

type mockUserClient struct {
	createUserFn      func(ctx context.Context, user users.User) (*users.User, error)
	getUserFn         func(ctx context.Context, id string) (*users.User, error)
	modifyUserFn      func(ctx context.Context, id string, user users.User) (*users.User, error)
	findUserByEmailFn func(ctx context.Context, email string) (*users.User, error)
	deleteUserFn      func(ctx context.Context, id string) error
}

func (m *mockUserClient) GetUser(ctx context.Context, id string) (*users.User, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, id)
	}
	return &users.User{ID: id}, nil
}

func (m *mockUserClient) CreateUser(ctx context.Context, user users.User) (*users.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, user)
	}
	return &users.User{}, nil
}

func (m *mockUserClient) ModifyUser(ctx context.Context, id string, user users.User) (*users.User, error) {
	if m.modifyUserFn != nil {
		return m.modifyUserFn(ctx, id, user)
	}
	return &users.User{}, nil
}

func (m *mockUserClient) DeleteUser(ctx context.Context, id string) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, id)
	}
	return nil
}

func (m *mockUserClient) FindUserByEmail(ctx context.Context, email string) (*users.User, error) {
	if m.findUserByEmailFn != nil {
		return m.findUserByEmailFn(ctx, email)
	}
	return nil, nil
}

func TestUserResource_Read_ReturnsErrorWithoutPanic(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return nil, errors.New("API unavailable")
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "some-uuid"),
		"first_name":           tftypes.NewValue(tftypes.String, nil),
		"last_name":            tftypes.NewValue(tftypes.String, nil),
		"email":                tftypes.NewValue(tftypes.String, nil),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{
			Schema: schema,
			Raw:    stateVal,
		},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{
			Schema: schema,
			Raw:    stateVal,
		},
	}

	r.Read(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
	if resp.Diagnostics.Errors()[0].Summary() != "Client Error" {
		t.Fatalf("unexpected error summary: %s", resp.Diagnostics.Errors()[0].Summary())
	}
}

func TestUserResource_Read_FallsBackToEmailLookup_WhenIDIsEmpty(t *testing.T) {
	findCalled := false
	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				t.Fatal("GetUser should not be called when ID is empty")
				return nil, nil
			},
			findUserByEmailFn: func(ctx context.Context, email string) (*users.User, error) {
				findCalled = true
				return &users.User{
					ID:        "found-uuid",
					FirstName: "John",
					LastName:  "Smith",
					Username:  email,
					Roles:     []users.UserRole{"DEVELOPER"},
				}, nil
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, ""),
		"first_name":           tftypes.NewValue(tftypes.String, nil),
		"last_name":            tftypes.NewValue(tftypes.String, nil),
		"email":                tftypes.NewValue(tftypes.String, "john@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !findCalled {
		t.Fatal("expected FindUserByEmail to be called, but it was not")
	}
}

func TestUserResource_Read_RemovesFromState_WhenIDIsEmptyAndUserNotFound(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				t.Fatal("GetUser should not be called when ID is empty")
				return nil, nil
			},
			findUserByEmailFn: func(ctx context.Context, email string) (*users.User, error) {
				return nil, nil
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, ""),
		"first_name":           tftypes.NewValue(tftypes.String, nil),
		"last_name":            tftypes.NewValue(tftypes.String, nil),
		"email":                tftypes.NewValue(tftypes.String, "john@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("expected resource to be removed from state, but state is not null")
	}
}

func TestUserResource_Read_RemovesFromState_WhenIDAndEmailAreEmpty(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				t.Fatal("GetUser should not be called when ID is empty")
				return nil, nil
			},
			findUserByEmailFn: func(ctx context.Context, email string) (*users.User, error) {
				t.Fatal("FindUserByEmail should not be called when email is empty")
				return nil, nil
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, ""),
		"first_name":           tftypes.NewValue(tftypes.String, nil),
		"last_name":            tftypes.NewValue(tftypes.String, nil),
		"email":                tftypes.NewValue(tftypes.String, ""),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("expected resource to be removed from state, but state is not null")
	}
}

func TestWaitForUser_RetriesUntilFound(t *testing.T) {
	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = time.Second
	apiLookupRetryPollInterval = 10 * time.Millisecond

	callCount := 0
	user, err := waitForUser(context.Background(), func() (*users.User, error) {
		callCount++
		if callCount < 3 {
			return nil, users.ErrNotFound
		}
		return &users.User{ID: "found-id"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if callCount != 3 {
		t.Fatalf("expected lookup to be retried until it succeeded (3 calls), got %d calls", callCount)
	}
	if user == nil || user.ID != "found-id" {
		t.Fatalf("expected to find user with ID %q, got %+v", "found-id", user)
	}
}

func TestWaitForUser_RetriesEmailLookupUntilFound(t *testing.T) {
	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = time.Second
	apiLookupRetryPollInterval = 10 * time.Millisecond

	callCount := 0
	user, err := waitForUser(context.Background(), func() (*users.User, error) {
		callCount++
		if callCount < 3 {
			return nil, nil // FindUserByEmail's not-found convention
		}
		return &users.User{ID: "found-id"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if callCount != 3 {
		t.Fatalf("expected lookup to be retried until it succeeded (3 calls), got %d calls", callCount)
	}
	if user == nil || user.ID != "found-id" {
		t.Fatalf("expected to find user with ID %q, got %+v", "found-id", user)
	}
}

func TestWaitForUser_ReturnsErrNotFound_WhenRetriesExhausted(t *testing.T) {
	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = 20 * time.Millisecond
	apiLookupRetryPollInterval = 5 * time.Millisecond

	user, err := waitForUser(context.Background(), func() (*users.User, error) {
		return nil, users.ErrNotFound
	})

	if !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("expected users.ErrNotFound once retries are exhausted, got %v", err)
	}
	if user != nil {
		t.Fatalf("expected nil user, got %+v", user)
	}
}

func TestWaitForUser_ReturnsNilNil_WhenEmailLookupNeverFinds(t *testing.T) {
	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = 20 * time.Millisecond
	apiLookupRetryPollInterval = 5 * time.Millisecond

	user, err := waitForUser(context.Background(), func() (*users.User, error) {
		return nil, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if user != nil {
		t.Fatalf("expected nil user, got %+v", user)
	}
}

func TestWaitForUser_ReturnsNonNotFoundErrorImmediately(t *testing.T) {
	originalTimeout := apiLookupRetryTimeout
	defer func() { apiLookupRetryTimeout = originalTimeout }()
	apiLookupRetryTimeout = time.Minute // would hang the test if not returned immediately

	callCount := 0
	wantErr := errors.New("API unavailable")
	_, err := waitForUser(context.Background(), func() (*users.User, error) {
		callCount++
		return nil, wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if callCount != 1 {
		t.Fatalf("expected lookup to be called exactly once (no retry) for a non-not-found error, got %d calls", callCount)
	}
}

func TestUserResource_Read_FallsBackToEmailLookup_WhenGetUserReturnsErrNotFound(t *testing.T) {
	findCalled := false
	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return nil, fmt.Errorf("wrapped: %w", users.ErrNotFound)
			},
			findUserByEmailFn: func(ctx context.Context, email string) (*users.User, error) {
				findCalled = true
				return &users.User{
					ID:        "new-user-id",
					FirstName: "John",
					LastName:  "Smith",
					Username:  email,
					Roles:     []users.UserRole{"DEVELOPER"},
				}, nil
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "stale-invitation-id"),
		"first_name":           tftypes.NewValue(tftypes.String, nil),
		"last_name":            tftypes.NewValue(tftypes.String, nil),
		"email":                tftypes.NewValue(tftypes.String, "john@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !findCalled {
		t.Fatal("expected FindUserByEmail to be called, but it was not")
	}

	var got UserResourceModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &got)...)
	if got.ID.ValueString() != "new-user-id" {
		t.Fatalf("expected state ID to be healed to %q, got %q", "new-user-id", got.ID.ValueString())
	}
}

func TestUserResource_Read_RemovesFromState_WhenGetUserReturnsErrNotFound_AndNoMatchingEmail(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return nil, users.ErrNotFound
			},
			findUserByEmailFn: func(ctx context.Context, email string) (*users.User, error) {
				return nil, nil
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "stale-invitation-id"),
		"first_name":           tftypes.NewValue(tftypes.String, nil),
		"last_name":            tftypes.NewValue(tftypes.String, nil),
		"email":                tftypes.NewValue(tftypes.String, "john@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("expected resource to be removed from state, but state is not null")
	}
}

func TestUserResource_ModifyPlan_SkipsReplaceCheck_WhenGetUserReturnsErrNotFound(t *testing.T) {
	r := UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return nil, users.ErrNotFound
			},
		},
	}

	schema := userResourceSchema()
	val := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "stale-invitation-id"),
		"first_name":           tftypes.NewValue(tftypes.String, "John"),
		"last_name":            tftypes.NewValue(tftypes.String, "Smith"),
		"email":                tftypes.NewValue(tftypes.String, "john@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, nil),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, nil),
	})

	req := resource.ModifyPlanRequest{
		State: tfsdk.State{Schema: schema, Raw: val},
		Plan:  tfsdk.Plan{Schema: schema, Raw: val},
	}
	resp := &resource.ModifyPlanResponse{
		Plan: tfsdk.Plan{Schema: schema, Raw: val},
	}

	r.ModifyPlan(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if len(resp.RequiresReplace) != 0 {
		t.Fatalf("expected no RequiresReplace paths, got %v", resp.RequiresReplace)
	}
}

func userResourceSchema() schema.Schema {
	r := &UserResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return schemaResp.Schema
}

func TestPopulateState_SetsVisibleAppsNullWhenAllAppsVisible(t *testing.T) {
	r := &UserResource{}
	data := &UserResourceModel{}
	var diags diag.Diagnostics

	user := &users.User{
		AllAppsVisible: true,
		VisibleAppIDs:  []string{"111", "222"},
	}

	r.populateState(context.Background(), data, user, diags)

	if !data.VisibleApps.IsNull() {
		t.Errorf("expected VisibleApps to be null when AllAppsVisible is true, got %v", data.VisibleApps)
	}
}

func TestPopulateState_SetsVisibleAppsFromAPIWhenNotAllAppsVisible(t *testing.T) {
	r := &UserResource{}
	data := &UserResourceModel{}
	var diags diag.Diagnostics

	user := &users.User{
		AllAppsVisible: false,
		VisibleAppIDs:  []string{"111", "222"},
	}

	r.populateState(context.Background(), data, user, diags)

	if data.VisibleApps.IsNull() {
		t.Error("expected VisibleApps to be populated when AllAppsVisible is false")
	}
	elements := data.VisibleApps.Elements()
	if len(elements) != 2 {
		t.Errorf("expected 2 visible app IDs, got %d", len(elements))
	}
}

func TestUserResource_Create_SetsVisibleAppsNullWhenAllAppsVisible(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			createUserFn: func(ctx context.Context, user users.User) (*users.User, error) {
				return &users.User{
					ID:             "new-uuid",
					AllAppsVisible: true,
					VisibleAppIDs:  []string{},
				}, nil
			},
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return &users.User{
					ID:             id,
					AllAppsVisible: true,
					VisibleAppIDs:  []string{},
				}, nil
			},
		},
	}

	schema := userResourceSchema()
	planVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, ""),
		"first_name":           tftypes.NewValue(tftypes.String, "Jo"),
		"last_name":            tftypes.NewValue(tftypes.String, "Dubey"),
		"email":                tftypes.NewValue(tftypes.String, "jodubey@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, true),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, false),
	})

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schema, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: schema, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data UserResourceModel
	resp.State.Get(context.Background(), &data)

	if !data.VisibleApps.IsNull() {
		t.Errorf("expected VisibleApps to be null when AllAppsVisible is true, got %v", data.VisibleApps)
	}
}

func TestUserResource_Create_WaitsUntilUserVisibleBeforeReturning(t *testing.T) {
	var getUserCalls int

	r := &UserResource{
		client: &mockUserClient{
			createUserFn: func(ctx context.Context, user users.User) (*users.User, error) {
				return &users.User{ID: "new-uuid"}, nil
			},
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				getUserCalls++
				if getUserCalls < 3 {
					return nil, users.ErrNotFound
				}
				return &users.User{ID: id}, nil
			},
		},
	}

	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = time.Second
	apiLookupRetryPollInterval = 10 * time.Millisecond

	schema := userResourceSchema()
	planVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, ""),
		"first_name":           tftypes.NewValue(tftypes.String, "Jo"),
		"last_name":            tftypes.NewValue(tftypes.String, "Dubey"),
		"email":                tftypes.NewValue(tftypes.String, "jodubey@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, true),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, false),
	})

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schema, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: schema, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if getUserCalls < 3 {
		t.Fatalf("expected Create to poll GetUser until visible, got %d call(s)", getUserCalls)
	}
}

func TestUserResource_Create_ErrorsWhenUserNeverBecomesVisible(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			createUserFn: func(ctx context.Context, user users.User) (*users.User, error) {
				return &users.User{ID: "new-uuid"}, nil
			},
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return nil, users.ErrNotFound
			},
		},
	}

	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = 20 * time.Millisecond
	apiLookupRetryPollInterval = 5 * time.Millisecond

	schema := userResourceSchema()
	planVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, ""),
		"first_name":           tftypes.NewValue(tftypes.String, "Jo"),
		"last_name":            tftypes.NewValue(tftypes.String, "Dubey"),
		"email":                tftypes.NewValue(tftypes.String, "jodubey@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, true),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, false),
	})

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: schema, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: schema, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
}

func TestUserResource_Update_SetsVisibleAppsNullWhenAllAppsVisible(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			modifyUserFn: func(ctx context.Context, id string, user users.User) (*users.User, error) {
				return &users.User{
					ID:             "some-uuid",
					AllAppsVisible: true,
					VisibleAppIDs:  []string{},
				}, nil
			},
		},
	}

	schema := userResourceSchema()
	planVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "some-uuid"),
		"first_name":           tftypes.NewValue(tftypes.String, "John"),
		"last_name":            tftypes.NewValue(tftypes.String, "Smith"),
		"email":                tftypes.NewValue(tftypes.String, "jsmith@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, true),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, nil),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, false),
	})

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schema, Raw: planVal},
		State: tfsdk.State{Schema: schema, Raw: planVal},
	}
	resp := &resource.UpdateResponse{
		State: tfsdk.State{Schema: schema, Raw: planVal},
	}

	r.Update(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data UserResourceModel
	resp.State.Get(context.Background(), &data)

	if !data.VisibleApps.IsNull() {
		t.Errorf("expected VisibleApps to be null when AllAppsVisible is true, got %v", data.VisibleApps)
	}
}

func TestUserResource_Update_DoesNotSendReadOnlyFields(t *testing.T) {
	var capturedUser users.User

	r := &UserResource{
		client: &mockUserClient{
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return &users.User{HasAcceptedInvite: true}, nil
			},
			modifyUserFn: func(ctx context.Context, id string, user users.User) (*users.User, error) {
				capturedUser = user
				return &users.User{}, nil
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "some-uuid"),
		"first_name":           tftypes.NewValue(tftypes.String, "John"),
		"last_name":            tftypes.NewValue(tftypes.String, "Smith"),
		"email":                tftypes.NewValue(tftypes.String, "jsmith@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, false),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{}),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, true),
	})

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: schema, Raw: stateVal},
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.UpdateResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Update(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if capturedUser.FirstName != "" {
		t.Errorf("expected FirstName to be empty in ModifyUser call, got %q", capturedUser.FirstName)
	}
	if capturedUser.LastName != "" {
		t.Errorf("expected LastName to be empty in ModifyUser call, got %q", capturedUser.LastName)
	}
	if capturedUser.Username != "" {
		t.Errorf("expected Username to be empty in ModifyUser call, got %q", capturedUser.Username)
	}
}

func TestUserResource_Delete_WaitsUntilUserDisappearsAfterSuccessfulDelete(t *testing.T) {
	var deleteCalled bool
	var getUserCalls int

	r := &UserResource{
		client: &mockUserClient{
			deleteUserFn: func(ctx context.Context, id string) error {
				deleteCalled = true
				return nil
			},
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				getUserCalls++
				if getUserCalls < 3 {
					return &users.User{ID: id}, nil
				}
				return nil, users.ErrNotFound
			},
		},
	}

	// Use a short poll interval in the test so it runs quickly, while still
	// exercising the real retry loop made by Delete().
	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = time.Second
	apiLookupRetryPollInterval = 10 * time.Millisecond

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "some-uuid"),
		"first_name":           tftypes.NewValue(tftypes.String, "John"),
		"last_name":            tftypes.NewValue(tftypes.String, "Smith"),
		"email":                tftypes.NewValue(tftypes.String, "jsmith@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, false),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{}),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, true),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Delete(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !deleteCalled {
		t.Fatal("expected DeleteUser to be called")
	}
	if getUserCalls < 3 {
		t.Fatalf("expected Delete to poll GetUser until not found, got %d call(s)", getUserCalls)
	}
}

func TestUserResource_Delete_ErrorsWhenUserNeverDisappears(t *testing.T) {
	r := &UserResource{
		client: &mockUserClient{
			deleteUserFn: func(ctx context.Context, id string) error {
				return nil
			},
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				return &users.User{ID: id}, nil
			},
		},
	}

	originalTimeout := apiLookupRetryTimeout
	originalPollInterval := apiLookupRetryPollInterval
	defer func() {
		apiLookupRetryTimeout = originalTimeout
		apiLookupRetryPollInterval = originalPollInterval
	}()
	apiLookupRetryTimeout = 20 * time.Millisecond
	apiLookupRetryPollInterval = 5 * time.Millisecond

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "some-uuid"),
		"first_name":           tftypes.NewValue(tftypes.String, "John"),
		"last_name":            tftypes.NewValue(tftypes.String, "Smith"),
		"email":                tftypes.NewValue(tftypes.String, "jsmith@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, false),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{}),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, true),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Delete(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
}

func TestUserResource_Delete_DoesNotWait_WhenDeleteUserFails(t *testing.T) {
	var getUserCalled bool

	r := &UserResource{
		client: &mockUserClient{
			deleteUserFn: func(ctx context.Context, id string) error {
				return errors.New("API unavailable")
			},
			getUserFn: func(ctx context.Context, id string) (*users.User, error) {
				getUserCalled = true
				return nil, users.ErrNotFound
			},
		},
	}

	schema := userResourceSchema()
	stateVal := tftypes.NewValue(schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, "some-uuid"),
		"first_name":           tftypes.NewValue(tftypes.String, "John"),
		"last_name":            tftypes.NewValue(tftypes.String, "Smith"),
		"email":                tftypes.NewValue(tftypes.String, "jsmith@example.com"),
		"roles":                tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "DEVELOPER")}),
		"all_apps_visible":     tftypes.NewValue(tftypes.Bool, false),
		"visible_apps":         tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{}),
		"provisioning_allowed": tftypes.NewValue(tftypes.Bool, true),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{
		State: tfsdk.State{Schema: schema, Raw: stateVal},
	}

	r.Delete(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
	if getUserCalled {
		t.Fatal("expected Delete NOT to poll GetUser when DeleteUser fails")
	}
}
