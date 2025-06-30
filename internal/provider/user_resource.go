// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
			},
			"last_name": schema.StringAttribute{
				MarkdownDescription: "User's last name",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "User's email address",
				Required:            true,
			},
			"roles": schema.SetAttribute{
				MarkdownDescription: "User's roles in the Apple Developer Program",
				ElementType:         types.StringType,
				Required:            true,
			},
			"all_apps_visible": schema.BoolAttribute{
				MarkdownDescription: "Whether the user can see all apps",
				Required:            true,
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

	user, err := r.client.CreateUser(ctx, users.User{
		FirstName:           data.FirstName.ValueString(),
		LastName:            data.LastName.ValueString(),
		Username:            data.Email.ValueString(),
		Roles:               roles,
		AllAppsVisible:      data.AllAppsVisible.ValueBool(),
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

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update example, got error: %s", err))
	//     return
	// }
	resp.Diagnostics.AddError("Update Not Supported", "The update operation is not supported for this resource. Please recreate the resource with the desired changes.")

	// Save updated data into Terraform state
	// resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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
