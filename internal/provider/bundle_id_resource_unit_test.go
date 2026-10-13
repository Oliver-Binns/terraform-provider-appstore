// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oliver-binns/appstore-go/bundleids"
	"github.com/oliver-binns/appstore-go/networking"
	"github.com/oliver-binns/appstore-go/openapi"
)

type mockBundleIDClient struct {
	getBundleIDFn              func(ctx context.Context, id string) (*bundleids.BundleID, error)
	findBundleIDByIdentifierFn func(ctx context.Context, identifier string) (*bundleids.BundleID, error)
	createBundleIDFn           func(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error)
	modifyBundleIDFn           func(ctx context.Context, id string, bundleID bundleids.BundleID) (*bundleids.BundleID, error)
	deleteBundleIDFn           func(ctx context.Context, id string) error
}

func (m *mockBundleIDClient) GetBundleID(ctx context.Context, id string) (*bundleids.BundleID, error) {
	if m.getBundleIDFn != nil {
		return m.getBundleIDFn(ctx, id)
	}
	return &bundleids.BundleID{}, nil
}

func (m *mockBundleIDClient) FindBundleIDByIdentifier(ctx context.Context, identifier string) (*bundleids.BundleID, error) {
	if m.findBundleIDByIdentifierFn != nil {
		return m.findBundleIDByIdentifierFn(ctx, identifier)
	}
	return nil, nil
}

func (m *mockBundleIDClient) CreateBundleID(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
	if m.createBundleIDFn != nil {
		return m.createBundleIDFn(ctx, bundleID)
	}
	return &bundleids.BundleID{}, nil
}

func (m *mockBundleIDClient) ModifyBundleID(ctx context.Context, id string, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
	if m.modifyBundleIDFn != nil {
		return m.modifyBundleIDFn(ctx, id, bundleID)
	}
	return &bundleids.BundleID{}, nil
}

func (m *mockBundleIDClient) DeleteBundleID(ctx context.Context, id string) error {
	if m.deleteBundleIDFn != nil {
		return m.deleteBundleIDFn(ctx, id)
	}
	return nil
}

func bundleIDResourceSchema() schema.Schema {
	r := &BundleIDResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return schemaResp.Schema
}

func bundleIDPlanVal(s schema.Schema) tftypes.Value {
	return tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, ""),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, nil),
	})
}

func TestBundleIDResource_Create_SetsStateCorrectly(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			createBundleIDFn: func(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{
					ID:         "bundle-uuid",
					Name:       "My App",
					Identifier: "com.example.myapp",
					Platform:   openapi.Universal,
					SeedID:     "ABCDE12345",
				}, nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{
					ID:         "bundle-uuid",
					Name:       "My App",
					Identifier: "com.example.myapp",
					Platform:   openapi.Universal,
					SeedID:     "ABCDE12345",
				}, nil
			},
		},
	}

	s := bundleIDResourceSchema()
	planVal := bundleIDPlanVal(s)

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: s, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: s, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data BundleIDResourceModel
	resp.State.Get(context.Background(), &data)

	if data.ID.ValueString() != "bundle-uuid" {
		t.Errorf("expected ID 'bundle-uuid', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "My App" {
		t.Errorf("expected Name 'My App', got %q", data.Name.ValueString())
	}
	if data.Identifier.ValueString() != "com.example.myapp" {
		t.Errorf("expected Identifier 'com.example.myapp', got %q", data.Identifier.ValueString())
	}
	if data.Platform.ValueString() != "UNIVERSAL" {
		t.Errorf("expected Platform 'IOS', got %q", data.Platform.ValueString())
	}
	if data.SeedID.ValueString() != "ABCDE12345" {
		t.Errorf("expected SeedID 'ABCDE12345', got %q", data.SeedID.ValueString())
	}
}

func TestBundleIDResource_Create_PassesSeedIDToClient(t *testing.T) {
	var capturedBundleID bundleids.BundleID

	r := &BundleIDResource{
		client: &mockBundleIDClient{
			createBundleIDFn: func(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
				capturedBundleID = bundleID
				return &bundleids.BundleID{
					ID:         "bundle-uuid",
					Name:       bundleID.Name,
					Identifier: bundleID.Identifier,
					Platform:   bundleID.Platform,
					SeedID:     bundleID.SeedID,
				}, nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{
					ID:         "bundle-uuid",
					Name:       "My App",
					Identifier: "com.example.myapp",
					Platform:   openapi.Universal,
					SeedID:     "ABCDE12345",
				}, nil
			},
		},
	}

	s := bundleIDResourceSchema()
	planVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, ""),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: s, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: s, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if capturedBundleID.SeedID != "ABCDE12345" {
		t.Errorf("expected SeedID 'ABCDE12345' passed to client, got %q", capturedBundleID.SeedID)
	}
}

func TestBundleIDResource_Create_ReportsClientError(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			createBundleIDFn: func(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
				return nil, errors.New("api error")
			},
		},
	}

	s := bundleIDResourceSchema()
	planVal := bundleIDPlanVal(s)

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: s, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: s, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostics, got none")
	}
}

func TestBundleIDResource_Create_WaitsUntilBundleIDVisibleBeforeReturning(t *testing.T) {
	var getBundleIDCalls int

	r := &BundleIDResource{
		client: &mockBundleIDClient{
			createBundleIDFn: func(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{ID: "bundle-uuid"}, nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				getBundleIDCalls++
				if getBundleIDCalls < 3 {
					return nil, networking.ErrNotFound
				}
				return &bundleids.BundleID{ID: id}, nil
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

	s := bundleIDResourceSchema()
	planVal := bundleIDPlanVal(s)

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: s, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: s, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if getBundleIDCalls < 3 {
		t.Fatalf("expected Create to poll GetBundleID until visible, got %d call(s)", getBundleIDCalls)
	}
}

func TestBundleIDResource_Create_ErrorsWhenBundleIDNeverBecomesVisible(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			createBundleIDFn: func(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{ID: "bundle-uuid"}, nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				return nil, networking.ErrNotFound
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

	s := bundleIDResourceSchema()
	planVal := bundleIDPlanVal(s)

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{Schema: s, Raw: planVal},
	}
	resp := &resource.CreateResponse{
		State: tfsdk.State{Schema: s, Raw: planVal},
	}

	r.Create(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
}

func TestBundleIDResource_Read_SetsStateCorrectly(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{
					ID:         id,
					Name:       "My App",
					Identifier: "com.example.myapp",
					Platform:   openapi.Universal,
					SeedID:     "ABCDE12345",
				}, nil
			},
		},
	}

	s := bundleIDResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.ReadRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.ReadResponse{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}

	r.Read(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data BundleIDResourceModel
	resp.State.Get(context.Background(), &data)

	if data.ID.ValueString() != "bundle-uuid" {
		t.Errorf("expected ID 'bundle-uuid', got %q", data.ID.ValueString())
	}
	if data.Identifier.ValueString() != "com.example.myapp" {
		t.Errorf("expected Identifier 'com.example.myapp', got %q", data.Identifier.ValueString())
	}
}

func TestBundleIDResource_Update_SetsNameCorrectly(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			modifyBundleIDFn: func(ctx context.Context, id string, bundleID bundleids.BundleID) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{
					ID:         id,
					Name:       bundleID.Name,
					Identifier: "com.example.myapp",
					Platform:   openapi.Universal,
					SeedID:     "ABCDE12345",
				}, nil
			},
		},
	}

	s := bundleIDResourceSchema()
	planVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App (renamed)"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: s, Raw: planVal},
		State: tfsdk.State{Schema: s, Raw: planVal},
	}
	resp := &resource.UpdateResponse{
		State: tfsdk.State{Schema: s, Raw: planVal},
	}

	r.Update(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data BundleIDResourceModel
	resp.State.Get(context.Background(), &data)

	if data.Name.ValueString() != "My App (renamed)" {
		t.Errorf("expected updated Name 'My App (renamed)', got %q", data.Name.ValueString())
	}
}

func TestBundleIDResource_Delete_CallsDeleteOnClient(t *testing.T) {
	var capturedID string

	r := &BundleIDResource{
		client: &mockBundleIDClient{
			deleteBundleIDFn: func(ctx context.Context, id string) error {
				capturedID = id
				return nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				return nil, networking.ErrNotFound
			},
		},
	}

	s := bundleIDResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if capturedID != "bundle-uuid" {
		t.Errorf("expected DeleteBundleID called with ID 'bundle-uuid', got %q", capturedID)
	}
}

func TestBundleIDResource_Delete_ReportsClientError(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			deleteBundleIDFn: func(ctx context.Context, id string) error {
				return errors.New("api error")
			},
		},
	}

	s := bundleIDResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostics, got none")
	}
}

func TestBundleIDResource_Delete_WaitsUntilBundleIDDisappearsAfterSuccessfulDelete(t *testing.T) {
	var deleteCalled bool
	var getBundleIDCalls int

	r := &BundleIDResource{
		client: &mockBundleIDClient{
			deleteBundleIDFn: func(ctx context.Context, id string) error {
				deleteCalled = true
				return nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				getBundleIDCalls++
				if getBundleIDCalls < 3 {
					return &bundleids.BundleID{ID: id}, nil
				}
				return nil, networking.ErrNotFound
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

	s := bundleIDResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if !deleteCalled {
		t.Fatal("expected DeleteBundleID to be called")
	}
	if getBundleIDCalls < 3 {
		t.Fatalf("expected Delete to poll GetBundleID until not found, got %d call(s)", getBundleIDCalls)
	}
}

func TestBundleIDResource_Delete_ErrorsWhenBundleIDNeverDisappears(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			deleteBundleIDFn: func(ctx context.Context, id string) error {
				return nil
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				return &bundleids.BundleID{ID: id}, nil
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

	s := bundleIDResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
}

func TestBundleIDResource_Delete_DoesNotWait_WhenDeleteBundleIDFails(t *testing.T) {
	var getBundleIDCalled bool

	r := &BundleIDResource{
		client: &mockBundleIDClient{
			deleteBundleIDFn: func(ctx context.Context, id string) error {
				return errors.New("API unavailable")
			},
			getBundleIDFn: func(ctx context.Context, id string) (*bundleids.BundleID, error) {
				getBundleIDCalled = true
				return nil, networking.ErrNotFound
			},
		},
	}

	s := bundleIDResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "bundle-uuid"),
		"name":       tftypes.NewValue(tftypes.String, "My App"),
		"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
		"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
		"seed_id":    tftypes.NewValue(tftypes.String, "ABCDE12345"),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic, got none")
	}
	if getBundleIDCalled {
		t.Fatal("expected Delete NOT to poll GetBundleID when DeleteBundleID fails")
	}
}

func TestBundleIDResource_ImportState_ByIdentifier(t *testing.T) {
	const testIdentifier = "com.example.myapp"

	r := &BundleIDResource{
		client: &mockBundleIDClient{
			findBundleIDByIdentifierFn: func(ctx context.Context, identifier string) (*bundleids.BundleID, error) {
				if identifier != testIdentifier {
					t.Errorf("expected identifier %q, got %q", testIdentifier, identifier)
				}
				return &bundleids.BundleID{
					ID:         "bundle-uuid",
					Name:       "My App",
					Identifier: identifier,
					Platform:   openapi.Universal,
					SeedID:     "ABCDE12345",
				}, nil
			},
		},
	}

	s := bundleIDResourceSchema()
	emptyVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, ""),
		"name":       tftypes.NewValue(tftypes.String, nil),
		"identifier": tftypes.NewValue(tftypes.String, nil),
		"platform":   tftypes.NewValue(tftypes.String, nil),
		"seed_id":    tftypes.NewValue(tftypes.String, nil),
	})

	req := resource.ImportStateRequest{ID: testIdentifier}
	resp := &resource.ImportStateResponse{
		State: tfsdk.State{Schema: s, Raw: emptyVal},
	}

	r.ImportState(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data BundleIDResourceModel
	resp.State.Get(context.Background(), &data)

	if data.ID.ValueString() != "bundle-uuid" {
		t.Errorf("expected ID 'bundle-uuid', got %q", data.ID.ValueString())
	}
	if data.Identifier.ValueString() != testIdentifier {
		t.Errorf("expected Identifier %q, got %q", testIdentifier, data.Identifier.ValueString())
	}
	if data.SeedID.ValueString() != "ABCDE12345" {
		t.Errorf("expected SeedID 'ABCDE12345', got %q", data.SeedID.ValueString())
	}
}

func TestBundleIDResource_ImportState_NotFound(t *testing.T) {
	r := &BundleIDResource{
		client: &mockBundleIDClient{
			findBundleIDByIdentifierFn: func(ctx context.Context, identifier string) (*bundleids.BundleID, error) {
				return nil, nil
			},
		},
	}

	s := bundleIDResourceSchema()
	emptyVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, ""),
		"name":       tftypes.NewValue(tftypes.String, nil),
		"identifier": tftypes.NewValue(tftypes.String, nil),
		"platform":   tftypes.NewValue(tftypes.String, nil),
		"seed_id":    tftypes.NewValue(tftypes.String, nil),
	})

	req := resource.ImportStateRequest{ID: "com.does.not.exist"}
	resp := &resource.ImportStateResponse{
		State: tfsdk.State{Schema: s, Raw: emptyVal},
	}

	r.ImportState(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostics for not-found bundle ID, got none")
	}
}

func TestBundleIDResource_ValidateConfig_Name(t *testing.T) {
	tests := map[string]struct {
		name    tftypes.Value
		wantErr bool
	}{
		"letters digits and spaces": {tftypes.NewValue(tftypes.String, "My App 2"), false},
		"allowed punctuation":       {tftypes.NewValue(tftypes.String, "Oliver's App-v1.0"), false},
		"parentheses":               {tftypes.NewValue(tftypes.String, "My App (renamed)"), true},
		"underscore":                {tftypes.NewValue(tftypes.String, "My_App"), true},
		"unknown":                   {tftypes.NewValue(tftypes.String, tftypes.UnknownValue), false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := bundleIDResourceSchema()
			raw := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
				"id":         tftypes.NewValue(tftypes.String, nil),
				"name":       tc.name,
				"identifier": tftypes.NewValue(tftypes.String, "com.example.myapp"),
				"platform":   tftypes.NewValue(tftypes.String, "UNIVERSAL"),
				"seed_id":    tftypes.NewValue(tftypes.String, nil),
			})

			resp := &resource.ValidateConfigResponse{}
			(&BundleIDResource{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{
				Config: tfsdk.Config{Schema: s, Raw: raw},
			}, resp)

			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("wantErr=%v, diagnostics: %v", tc.wantErr, resp.Diagnostics)
			}
		})
	}
}
