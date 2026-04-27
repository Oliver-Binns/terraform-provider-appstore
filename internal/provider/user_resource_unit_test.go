// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oliver-binns/appstore-go/users"
)

type mockUserClient struct {
	getUserFn func(ctx context.Context, id string) (*users.User, error)
}

func (m *mockUserClient) GetUser(ctx context.Context, id string) (*users.User, error) {
	return m.getUserFn(ctx, id)
}

func (m *mockUserClient) CreateUser(ctx context.Context, user users.User) (*users.User, error) {
	return nil, nil
}

func (m *mockUserClient) ModifyUser(ctx context.Context, id string, user users.User) (*users.User, error) {
	return nil, nil
}

func (m *mockUserClient) DeleteUser(ctx context.Context, id string) error {
	return nil
}

func (m *mockUserClient) FindUserByEmail(ctx context.Context, email string) (*users.User, error) {
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

func userResourceSchema() schema.Schema {
	r := &UserResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return schemaResp.Schema
}
