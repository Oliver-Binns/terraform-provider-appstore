// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/oliver-binns/appstore-go/devices"
	"github.com/oliver-binns/appstore-go/openapi"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DeviceResource{}
var _ resource.ResourceWithImportState = &DeviceResource{}

type deviceClient interface {
	FindDeviceByUDID(ctx context.Context, udid string) (*devices.Device, error)
	GetDevice(ctx context.Context, id string) (*devices.Device, error)
	RegisterDevice(ctx context.Context, device devices.Device) (*devices.Device, error)
	ModifyDevice(ctx context.Context, id string, device devices.Device) (*devices.Device, error)
}

func NewDeviceResource() resource.Resource {
	return &DeviceResource{}
}

// DeviceResource defines the resource implementation.
type DeviceResource struct {
	client deviceClient
}

// DeviceResourceModel describes the resource data model.
type DeviceResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	UDID        types.String `tfsdk:"udid"`
	Platform    types.String `tfsdk:"platform"`
	DeviceClass types.String `tfsdk:"device_class"`
	Model       types.String `tfsdk:"model"`
	Status      types.String `tfsdk:"status"`
}

func (r *DeviceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (r *DeviceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a device registered in App Store Connect.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the device.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the device.",
			},
			"udid": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The device's unique device identifier (UDID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The platform of the device (e.g. `IOS`, `MAC_OS`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"device_class": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The class of the device as determined by Apple (e.g. `IPHONE`, `IPAD`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"model": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The model of the device as determined by Apple.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The status of the device: `ENABLED` or `DISABLED`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *DeviceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(deviceClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected deviceClient, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *DeviceResource) populateState(data *DeviceResourceModel, device *devices.Device) {
	data.ID = types.StringValue(device.ID)
	data.Name = types.StringValue(device.Name)
	data.UDID = types.StringValue(device.UDID)
	data.Platform = types.StringValue(string(device.Platform))
	data.DeviceClass = types.StringValue(string(device.DeviceClass))
	data.Model = types.StringValue(device.Model)
	data.Status = types.StringValue(string(device.Status))
}

func (r *DeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DeviceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	device, err := r.client.RegisterDevice(ctx, devices.Device{
		Name:     data.Name.ValueString(),
		UDID:     data.UDID.ValueString(),
		Platform: openapi.BundleIdPlatform(data.Platform.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to register device, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "registered a new device")

	r.populateState(&data, device)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DeviceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	device, err := r.client.GetDevice(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read device, got error: %s", err))
		return
	}

	r.populateState(&data, device)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DeviceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	device, err := r.client.ModifyDevice(ctx, data.ID.ValueString(), devices.Device{
		Name:   data.Name.ValueString(),
		Status: openapi.DeviceStatus(data.Status.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to modify device, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "modified a device")

	r.populateState(&data, device)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DeviceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.ModifyDevice(ctx, data.ID.ValueString(), devices.Device{
		Name:   data.Name.ValueString(),
		Status: openapi.Disabled,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to disable device, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "disabled device on destroy")
}

func (r *DeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data DeviceResourceModel

	device, err := r.client.FindDeviceByUDID(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to import device, got error: %s", err))
		return
	}
	if device == nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("No device found with UDID %q", req.ID))
		return
	}

	r.populateState(&data, device)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
