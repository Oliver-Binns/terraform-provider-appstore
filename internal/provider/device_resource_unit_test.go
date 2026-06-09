// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/oliver-binns/appstore-go/devices"
	"github.com/oliver-binns/appstore-go/openapi"
)

type mockDeviceClient struct {
	registerDeviceFn   func(ctx context.Context, device devices.Device) (*devices.Device, error)
	getDeviceFn        func(ctx context.Context, id string) (*devices.Device, error)
	modifyDeviceFn     func(ctx context.Context, id string, device devices.Device) (*devices.Device, error)
	findDeviceByUDIDFn func(ctx context.Context, udid string) (*devices.Device, error)
}

func (m *mockDeviceClient) FindDeviceByUDID(ctx context.Context, udid string) (*devices.Device, error) {
	if m.findDeviceByUDIDFn != nil {
		return m.findDeviceByUDIDFn(ctx, udid)
	}
	return nil, nil
}

func (m *mockDeviceClient) RegisterDevice(ctx context.Context, device devices.Device) (*devices.Device, error) {
	if m.registerDeviceFn != nil {
		return m.registerDeviceFn(ctx, device)
	}
	return &devices.Device{}, nil
}

func (m *mockDeviceClient) GetDevice(ctx context.Context, id string) (*devices.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}
	return &devices.Device{}, nil
}

func (m *mockDeviceClient) ModifyDevice(ctx context.Context, id string, device devices.Device) (*devices.Device, error) {
	if m.modifyDeviceFn != nil {
		return m.modifyDeviceFn(ctx, id, device)
	}
	return &devices.Device{}, nil
}

func deviceResourceSchema() schema.Schema {
	r := &DeviceResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	return schemaResp.Schema
}

func devicePlanVal(s schema.Schema) tftypes.Value {
	return tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, ""),
		"name":         tftypes.NewValue(tftypes.String, "Oliver's iPhone"),
		"udid":         tftypes.NewValue(tftypes.String, "00008101-001234AB3C04001E"),
		"platform":     tftypes.NewValue(tftypes.String, "IOS"),
		"device_class": tftypes.NewValue(tftypes.String, nil),
		"model":        tftypes.NewValue(tftypes.String, nil),
		"status":       tftypes.NewValue(tftypes.String, nil),
	})
}

func TestDeviceResource_Create_SetsStateCorrectly(t *testing.T) {
	r := &DeviceResource{
		client: &mockDeviceClient{
			registerDeviceFn: func(ctx context.Context, device devices.Device) (*devices.Device, error) {
				return &devices.Device{
					ID:          "device-uuid",
					Name:        "Oliver's iPhone",
					UDID:        "00008101-001234AB3C04001E",
					Platform:    openapi.IOS,
					DeviceClass: openapi.IPHONE,
					Model:       "iPhone 14 Pro",
					Status:      openapi.Enabled,
				}, nil
			},
		},
	}

	s := deviceResourceSchema()
	planVal := devicePlanVal(s)

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

	var data DeviceResourceModel
	resp.State.Get(context.Background(), &data)

	if data.ID.ValueString() != "device-uuid" {
		t.Errorf("expected ID 'device-uuid', got %q", data.ID.ValueString())
	}
	if data.Name.ValueString() != "Oliver's iPhone" {
		t.Errorf("expected Name 'Oliver's iPhone', got %q", data.Name.ValueString())
	}
	if data.UDID.ValueString() != "00008101-001234AB3C04001E" {
		t.Errorf("expected UDID '00008101-001234AB3C04001E', got %q", data.UDID.ValueString())
	}
	if data.Platform.ValueString() != "IOS" {
		t.Errorf("expected Platform 'IOS', got %q", data.Platform.ValueString())
	}
	if data.DeviceClass.ValueString() != "IPHONE" {
		t.Errorf("expected DeviceClass 'IPHONE', got %q", data.DeviceClass.ValueString())
	}
	if data.Model.ValueString() != "iPhone 14 Pro" {
		t.Errorf("expected Model 'iPhone 14 Pro', got %q", data.Model.ValueString())
	}
	if data.Status.ValueString() != "ENABLED" {
		t.Errorf("expected Status 'ENABLED', got %q", data.Status.ValueString())
	}
}

func TestDeviceResource_Update_SetsStateCorrectly(t *testing.T) {
	r := &DeviceResource{
		client: &mockDeviceClient{
			modifyDeviceFn: func(ctx context.Context, id string, device devices.Device) (*devices.Device, error) {
				return &devices.Device{
					ID:          id,
					Name:        device.Name,
					UDID:        "00008101-001234AB3C04001E",
					Platform:    openapi.IOS,
					DeviceClass: openapi.IPHONE,
					Model:       "iPhone 14 Pro",
					Status:      device.Status,
				}, nil
			},
		},
	}

	s := deviceResourceSchema()
	planVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "device-uuid"),
		"name":         tftypes.NewValue(tftypes.String, "Oliver's iPhone (renamed)"),
		"udid":         tftypes.NewValue(tftypes.String, "00008101-001234AB3C04001E"),
		"platform":     tftypes.NewValue(tftypes.String, "IOS"),
		"device_class": tftypes.NewValue(tftypes.String, "IPHONE"),
		"model":        tftypes.NewValue(tftypes.String, "iPhone 14 Pro"),
		"status":       tftypes.NewValue(tftypes.String, "ENABLED"),
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

	var data DeviceResourceModel
	resp.State.Get(context.Background(), &data)

	if data.Name.ValueString() != "Oliver's iPhone (renamed)" {
		t.Errorf("expected updated Name, got %q", data.Name.ValueString())
	}
	if data.Status.ValueString() != "ENABLED" {
		t.Errorf("expected Status 'ENABLED', got %q", data.Status.ValueString())
	}
}

func TestDeviceResource_Delete_DisablesDevice(t *testing.T) {
	var capturedDevice devices.Device
	var capturedID string

	r := &DeviceResource{
		client: &mockDeviceClient{
			modifyDeviceFn: func(ctx context.Context, id string, device devices.Device) (*devices.Device, error) {
				capturedID = id
				capturedDevice = device
				return &devices.Device{ID: id, Status: device.Status}, nil
			},
		},
	}

	s := deviceResourceSchema()
	stateVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, "device-uuid"),
		"name":         tftypes.NewValue(tftypes.String, "Oliver's iPhone"),
		"udid":         tftypes.NewValue(tftypes.String, "00008101-001234AB3C04001E"),
		"platform":     tftypes.NewValue(tftypes.String, "IOS"),
		"device_class": tftypes.NewValue(tftypes.String, "IPHONE"),
		"model":        tftypes.NewValue(tftypes.String, "iPhone 14 Pro"),
		"status":       tftypes.NewValue(tftypes.String, "ENABLED"),
	})

	req := resource.DeleteRequest{
		State: tfsdk.State{Schema: s, Raw: stateVal},
	}
	resp := &resource.DeleteResponse{}

	r.Delete(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}
	if capturedID != "device-uuid" {
		t.Errorf("expected ModifyDevice called with ID 'device-uuid', got %q", capturedID)
	}
	if capturedDevice.Status != openapi.Disabled {
		t.Errorf("expected ModifyDevice called with Status DISABLED, got %q", capturedDevice.Status)
	}
}

func TestDeviceResource_ImportState_ByUDID(t *testing.T) {
	r := &DeviceResource{
		client: &mockDeviceClient{
			findDeviceByUDIDFn: func(ctx context.Context, udid string) (*devices.Device, error) {
				if udid != iphone16ProUDID {
					t.Errorf("expected UDID %q, got %q", iphone16ProUDID, udid)
				}
				return &devices.Device{
					ID:          "device-uuid",
					Name:        iphone16ProName,
					UDID:        udid,
					Platform:    openapi.IOS,
					DeviceClass: openapi.IPHONE,
					Model:       "iPhone 16 Pro",
					Status:      openapi.Enabled,
				}, nil
			},
		},
	}

	s := deviceResourceSchema()
	emptyVal := tftypes.NewValue(s.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"id":           tftypes.NewValue(tftypes.String, ""),
		"name":         tftypes.NewValue(tftypes.String, nil),
		"udid":         tftypes.NewValue(tftypes.String, nil),
		"platform":     tftypes.NewValue(tftypes.String, nil),
		"device_class": tftypes.NewValue(tftypes.String, nil),
		"model":        tftypes.NewValue(tftypes.String, nil),
		"status":       tftypes.NewValue(tftypes.String, nil),
	})

	req := resource.ImportStateRequest{ID: iphone16ProUDID}
	resp := &resource.ImportStateResponse{
		State: tfsdk.State{Schema: s, Raw: emptyVal},
	}

	r.ImportState(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected error: %s", resp.Diagnostics.Errors()[0].Detail())
	}

	var data DeviceResourceModel
	resp.State.Get(context.Background(), &data)

	if data.ID.ValueString() != "device-uuid" {
		t.Errorf("expected ID 'device-uuid', got %q", data.ID.ValueString())
	}
	if data.UDID.ValueString() != iphone16ProUDID {
		t.Errorf("expected UDID %q, got %q", iphone16ProUDID, data.UDID.ValueString())
	}
}
