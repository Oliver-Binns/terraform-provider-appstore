// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/oliver-binns/appstore-go"
	"github.com/oliver-binns/appstore-go/users"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

// UserResource defines the resource implementation.
type UserResource struct {
	client *appstore.Client
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	ID                  types.String `tfsdk:"id"` // Computed attribute, used for the resource ID
	FirstName           types.String `tfsdk:"first_name"`
	LastName            types.String `tfsdk:"last_name"`
	Email               types.String `tfsdk:"email"`
	Roles               types.Set    `tfsdk:"roles"`
	AllAppsVisible      types.Bool   `tfsdk:"all_apps_visible"`
	VisibleApps         types.Set    `tfsdk:"visible_apps"`
	ProvisioningAllowed types.Bool   `tfsdk:"provisioning_allowed"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Manage users in the Apple Developer Program using the App Store Connect API.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User identifier",
			},
			"first_name": schema.StringAttribute{
				MarkdownDescription: "User's first name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"last_name": schema.StringAttribute{
				MarkdownDescription: "User's last name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "User's email address",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"roles": schema.SetAttribute{
				MarkdownDescription: "User's roles in the Apple Developer Program",
				ElementType:         types.StringType,
				Required:            true,
			},
			"all_apps_visible": schema.BoolAttribute{
				MarkdownDescription: "Whether the user can see all apps",
				Optional:            true,
			},
			"visible_apps": schema.SetAttribute{
				MarkdownDescription: "A list of IDs for the apps that the user has permission to see",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"provisioning_allowed": schema.BoolAttribute{
				MarkdownDescription: "Whether the user is allowed to create new provisioning profiles",
				Required:            true,
			},
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*appstore.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *appstore.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r UserResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If the user can view all apps, the list of apps must not be set
	if data.AllAppsVisible.ValueBool() && !data.VisibleApps.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("visible_apps"),
			"Invalid Configuration",
			"If `all_apps_visible` is set to true, the list of visible apps must not be provided.",
		)
		return
	}
}

func (r UserResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// If the resource is being modified (neither created, nor destroyed):
	if !req.State.Raw.IsNull() && !req.Plan.Raw.IsNull() {
		// Read Terraform state data into a model
		var data UserResourceModel
		resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

		// Check if the user has accepted their email invite yet:
		user, err := r.client.GetUser(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Fetch error",
				fmt.Sprintf("Unable to fetch state of user %s, got error: %s", data.ID.ValueString(), err),
			)
			return
		}

		// If not, we must replace the resource:
		if !user.HasAcceptedInvite {
			for p := range req.State.Schema.GetAttributes() {
				resp.RequiresReplace = append(resp.RequiresReplace, path.Root(p))
			}

			resp.Diagnostics.AddWarning(
				"Cannot modify user",
				"Users cannot be modified until they have accepted their email invite to App Store Connect. This resource will be destroyed and recreated.",
			)
		}
	}
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roles := []users.UserRole{}
	diag := data.Roles.ElementsAs(ctx, &roles, false)
	resp.Diagnostics.Append(diag...)

	appIDs := []string{}
	diag = data.VisibleApps.ElementsAs(ctx, &appIDs, false)
	resp.Diagnostics.Append(diag...)

	user, err := r.client.CreateUser(ctx, users.User{
		FirstName:           data.FirstName.ValueString(),
		LastName:            data.LastName.ValueString(),
		Username:            data.Email.ValueString(),
		Roles:               roles,
		AllAppsVisible:      data.AllAppsVisible.ValueBool(),
		VisibleAppIDs:       appIDs,
		ProvisioningAllowed: data.ProvisioningAllowed.ValueBool(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create user, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "created a new user")

	data.ID = types.StringValue(user.ID)
	data.FirstName = types.StringValue(user.FirstName)
	data.LastName = types.StringValue(user.LastName)
	data.Email = types.StringValue(user.Username)

	data.Roles, diag = types.SetValueFrom(ctx, types.StringType, user.Roles)
	resp.Diagnostics.Append(diag...)

	data.VisibleApps, diag = types.SetValueFrom(ctx, types.StringType, user.VisibleAppIDs)
	resp.Diagnostics.Append(diag...)

	data.AllAppsVisible = types.BoolValue(user.AllAppsVisible)
	data.ProvisioningAllowed = types.BoolValue(user.ProvisioningAllowed)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, data.ID.ValueString())

	data.ID = types.StringValue(user.ID)
	data.FirstName = types.StringValue(user.FirstName)
	data.LastName = types.StringValue(user.LastName)
	data.Email = types.StringValue(user.Username)

	roles, diag := types.SetValueFrom(ctx, types.StringType, user.Roles)
	data.Roles = roles
	resp.Diagnostics.Append(diag...)

	data.VisibleApps, diag = types.SetValueFrom(ctx, types.StringType, user.VisibleAppIDs)
	resp.Diagnostics.Append(diag...)

	data.AllAppsVisible = types.BoolValue(user.AllAppsVisible)
	data.ProvisioningAllowed = types.BoolValue(user.ProvisioningAllowed)

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roles := []users.UserRole{}
	diag := data.Roles.ElementsAs(ctx, &roles, false)
	resp.Diagnostics.Append(diag...)

	appIDs := []string{}
	diag = data.VisibleApps.ElementsAs(ctx, &appIDs, false)
	resp.Diagnostics.Append(diag...)

	user, err := r.client.ModifyUser(ctx, data.ID.ValueString(), users.User{
		FirstName:           data.FirstName.ValueString(),
		LastName:            data.LastName.ValueString(),
		Username:            data.Email.ValueString(),
		Roles:               roles,
		AllAppsVisible:      data.AllAppsVisible.ValueBool(),
		VisibleAppIDs:       appIDs,
		ProvisioningAllowed: data.ProvisioningAllowed.ValueBool(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to modify user, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "modified a user")

	data.ID = types.StringValue(user.ID)
	data.FirstName = types.StringValue(user.FirstName)
	data.LastName = types.StringValue(user.LastName)
	data.Email = types.StringValue(user.Username)

	data.Roles, diag = types.SetValueFrom(ctx, types.StringType, user.Roles)
	resp.Diagnostics.Append(diag...)

	data.VisibleApps, diag = types.SetValueFrom(ctx, types.StringType, user.VisibleAppIDs)
	resp.Diagnostics.Append(diag...)

	data.AllAppsVisible = types.BoolValue(user.AllAppsVisible)
	data.ProvisioningAllowed = types.BoolValue(user.ProvisioningAllowed)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteUser(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user, got error: %s", err))
		return
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
