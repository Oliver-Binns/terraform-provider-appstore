// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/oliver-binns/appstore-go/bundleids"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BundleIDResource{}
var _ resource.ResourceWithImportState = &BundleIDResource{}
var _ resource.ResourceWithValidateConfig = &BundleIDResource{}

// App Store Connect only accepts letters, digits, spaces and . - ' in bundle ID names.
var bundleIDNamePattern = regexp.MustCompile(`^[\p{L}\p{N} .'-]+$`)

type bundleIDClient interface {
	GetBundleID(ctx context.Context, id string) (*bundleids.BundleID, error)
	FindBundleIDByIdentifier(ctx context.Context, identifier string) (*bundleids.BundleID, error)
	CreateBundleID(ctx context.Context, bundleID bundleids.BundleID) (*bundleids.BundleID, error)
	ModifyBundleID(ctx context.Context, id string, bundleID bundleids.BundleID) (*bundleids.BundleID, error)
	DeleteBundleID(ctx context.Context, id string) error
}

func NewBundleIDResource() resource.Resource {
	return &BundleIDResource{}
}

// BundleIDResource defines the resource implementation.
type BundleIDResource struct {
	client bundleIDClient
}

// BundleIDResourceModel describes the resource data model.
type BundleIDResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Identifier types.String `tfsdk:"identifier"`
	Platform   types.String `tfsdk:"platform"`
	SeedID     types.String `tfsdk:"seed_id"`
}

func (r *BundleIDResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bundle_id"
}

func (r *BundleIDResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a bundle ID registered in App Store Connect.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the bundle ID resource (Apple-assigned, not the identifier string).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the bundle ID. May only contain letters, numbers, spaces and the characters `.`, `-` and `'`.",
			},
			"identifier": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The bundle ID identifier string (e.g. `com.example.app`). Immutable after creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The platform for the bundle ID: `IOS`, `MAC_OS`, or `UNIVERSAL`. Immutable after creation — the App Store Connect API may normalize the value (e.g. `IOS` becomes `UNIVERSAL`). To recreate with a different platform, remove and re-add the resource.",
			},
			"seed_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The team's seed ID assigned to the bundle ID. Assigned by Apple if not specified.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *BundleIDResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var name types.String

	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() || name.IsNull() || name.IsUnknown() {
		return
	}

	if !bundleIDNamePattern.MatchString(name.ValueString()) {
		resp.Diagnostics.AddAttributeError(
			path.Root("name"),
			"Invalid Bundle ID Name",
			"The name may only contain letters, numbers, spaces and the characters `.`, `-` and `'`.",
		)
	}
}

func (r *BundleIDResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(bundleIDClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected bundleIDClient, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *BundleIDResource) populateState(data *BundleIDResourceModel, bundleID *bundleids.BundleID) {
	data.ID = types.StringValue(bundleID.ID)
	data.Name = types.StringValue(bundleID.Name)
	data.Identifier = types.StringValue(bundleID.Identifier)
	data.Platform = types.StringValue(string(bundleID.Platform))
	data.SeedID = types.StringValue(bundleID.SeedID)
}

func (r *BundleIDResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BundleIDResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bundleID, err := r.client.CreateBundleID(ctx, bundleids.BundleID{
		Name:       data.Name.ValueString(),
		Identifier: data.Identifier.ValueString(),
		Platform:   bundleids.Platform(data.Platform.ValueString()),
		SeedID:     data.SeedID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create bundle ID, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created a new bundle ID")

	// Wait for eventual consistency before the implicit post-apply refresh reads this back.
	bundleID, err = waitForResource(ctx, func() (*bundleids.BundleID, error) {
		return r.client.GetBundleID(ctx, bundleID.ID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Bundle ID was created but did not become visible in time: %s", err))
		return
	}

	r.populateState(&data, bundleID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BundleIDResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BundleIDResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bundleID, err := r.client.GetBundleID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read bundle ID, got error: %s", err))
		return
	}

	r.populateState(&data, bundleID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BundleIDResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BundleIDResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bundleID, err := r.client.ModifyBundleID(ctx, data.ID.ValueString(), bundleids.BundleID{
		Name: data.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to modify bundle ID, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "modified a bundle ID")

	r.populateState(&data, bundleID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BundleIDResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BundleIDResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteBundleID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete bundle ID, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a bundle ID")

	// Wait so an immediate recreate (e.g. forced replace) doesn't 409 on the reused identifier.
	if err := waitForResourceDeleted(ctx, func() (*bundleids.BundleID, error) {
		return r.client.GetBundleID(ctx, data.ID.ValueString())
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Bundle ID was deleted but did not disappear from the API in time: %s", err))
		return
	}
}

func (r *BundleIDResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data BundleIDResourceModel

	bundleID, err := r.client.FindBundleIDByIdentifier(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to import bundle ID, got error: %s", err))
		return
	}
	if bundleID == nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("No bundle ID found with identifier %q", req.ID))
		return
	}

	r.populateState(&data, bundleID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
