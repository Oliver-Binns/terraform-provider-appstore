// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oliver-binns/appstore-go/users"
)

type mockUserClient struct {
	getUserFn         func(ctx context.Context, id string) (*users.User, error)
	modifyUserFn      func(ctx context.Context, id string, user users.User) (*users.User, error)
	findUserByEmailFn func(ctx context.Context, email string) (*users.User, error)
}

func (m *mockUserClient) GetUser(ctx context.Context, id string) (*users.User, error) {
	return m.getUserFn(ctx, id)
}

func (m *mockUserClient) CreateUser(ctx context.Context, user users.User) (*users.User, error) {
	return nil, nil
}

func (m *mockUserClient) ModifyUser(ctx context.Context, id string, user users.User) (*users.User, error) {
	if m.modifyUserFn != nil {
		return m.modifyUserFn(ctx, id, user)
	}
	return &users.User{}, nil
}

func (m *mockUserClient) DeleteUser(ctx context.Context, id string) error {
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
