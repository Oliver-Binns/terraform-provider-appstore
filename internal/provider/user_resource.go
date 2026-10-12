// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/oliver-binns/appstore-go"
	"github.com/oliver-binns/appstore-go/users"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

// userLookupRetryTimeout and userLookupRetryPollInterval account for
// eventual consistency in the App Store Connect API: a user invitation
// created or replaced moments ago may not be immediately resolvable via
// GetUser or FindUserByEmail. They are vars (rather than consts) so tests
// can shorten them.
var userLookupRetryTimeout = 30 * time.Second
var userLookupRetryPollInterval = 2 * time.Second

// userFoundState and userNotFoundState label the two states a user lookup
// can resolve to, shared by waitForUser (polls until found) and
// waitForUserDeleted (polls until not found).
const (
	userFoundState    = "found"
	userNotFoundState = "not-found"
)

// classifyUserLookup adapts lookup into a retry.StateChangeConf Refresh
// function, tagging each result as userFoundState or userNotFoundState.
// Not-found is recognised two ways, matching callers' existing
// conventions: a GetUser-style lookup returning users.ErrNotFound, or a
// FindUserByEmail-style lookup returning (nil, nil). lastErr records the
// most recent not-found error (nil for the FindUserByEmail convention) so
// waitForUser can restore the exact not-found signal once retries are
// exhausted. Any other error is surfaced immediately, unretried.
func classifyUserLookup(lookup func() (*users.User, error), lastErr *error) func() (interface{}, string, error) {
	return func() (interface{}, string, error) {
		user, err := lookup()
		if err != nil {
			if errors.Is(err, users.ErrNotFound) {
				*lastErr = err
				return struct{}{}, userNotFoundState, nil
			}
			return nil, "", err
		}
		if user == nil {
			*lastErr = nil
			return struct{}{}, userNotFoundState, nil
		}
		return user, userFoundState, nil
	}
}

// waitForUser retries lookup while it reports the user as not found,
// tolerating eventual consistency in the App Store Connect API, up to
// userLookupRetryTimeout. It preserves the exact not-found signal callers
// already expect: a GetUser-style lookup that keeps returning
// users.ErrNotFound still returns that error once retries are exhausted; a
// FindUserByEmail-style lookup that keeps returning (nil, nil) still
// returns (nil, nil). Any other error is returned immediately, unretried.
func waitForUser(ctx context.Context, lookup func() (*users.User, error)) (*users.User, error) {
	var lastErr error
	stateConf := &retry.StateChangeConf{
		Pending:    []string{userNotFoundState},
		Target:     []string{userFoundState},
		Refresh:    classifyUserLookup(lookup, &lastErr),
		Timeout:    userLookupRetryTimeout,
		MinTimeout: userLookupRetryPollInterval,
	}

	result, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		var timeoutErr *retry.TimeoutError
		if errors.As(err, &timeoutErr) {
			return nil, lastErr
		}
		return nil, err
	}
	user, ok := result.(*users.User)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T returned while waiting for user", result)
	}
	return user, nil
}

// waitForUserDeleted polls lookup until it reports the user as not found
// (users.ErrNotFound), tolerating eventual consistency in the App Store
// Connect API: revoking a user invitation does not always immediately
// release its email address for reuse, which otherwise risks a 409 "email
// already in use" error if Terraform immediately recreates a resource using
// the same email (e.g. as part of a destroy-then-create replace). Any other
// error from lookup is returned immediately, unretried.
func waitForUserDeleted(ctx context.Context, lookup func() (*users.User, error)) error {
	var lastErr error
	stateConf := &retry.StateChangeConf{
		Pending:    []string{userFoundState},
		Target:     []string{userNotFoundState},
		Refresh:    classifyUserLookup(lookup, &lastErr),
		Timeout:    userLookupRetryTimeout,
		MinTimeout: userLookupRetryPollInterval,
	}

	_, err := stateConf.WaitForStateContext(ctx)
	return err
}

type userClient interface {
	GetUser(ctx context.Context, id string) (*users.User, error)
	CreateUser(ctx context.Context, user users.User) (*users.User, error)
	ModifyUser(ctx context.Context, id string, user users.User) (*users.User, error)
	DeleteUser(ctx context.Context, id string) error
	FindUserByEmail(ctx context.Context, email string) (*users.User, error)
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

// UserResource defines the resource implementation.
type UserResource struct {
	client userClient
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

	// Wait for the newly created user to become visible via GetUser before
	// returning: the App Store Connect API is eventually consistent, and
	// Terraform's implicit post-apply refresh (Read/ModifyPlan) can
	// otherwise see a stale 404 moments after a successful create. Use the
	// confirmed GetUser response to populate state, so it matches what a
	// subsequent Read will see.
	user, err = waitForUser(ctx, func() (*users.User, error) {
		return r.client.GetUser(ctx, user.ID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("User was created but did not become visible in time: %s", err))
		return
	}

	r.populateState(ctx, &data, user, resp.Diagnostics)

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

	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		if data.Email.IsNull() || data.Email.IsUnknown() || data.Email.ValueString() == "" {
			tflog.Warn(ctx, "User resource has no ID or email in state, removing from state to allow re-import")
			resp.State.RemoveResource(ctx)
			return
		}
		user, err := r.client.FindUserByEmail(ctx, data.Email.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find user by email, got error: %s", err))
			return
		}
		if user == nil {
			resp.State.RemoveResource(ctx)
			return
		}
		r.populateState(ctx, &data, user, resp.Diagnostics)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	user, err := r.client.GetUser(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user, got error: %s", err))
		return
	}

	r.populateState(ctx, &data, user, resp.Diagnostics)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) populateState(ctx context.Context, data *UserResourceModel, user *users.User, diags diag.Diagnostics) {
	data.ID = types.StringValue(user.ID)
	data.FirstName = types.StringValue(user.FirstName)
	data.LastName = types.StringValue(user.LastName)
	data.Email = types.StringValue(user.Username)

	var d diag.Diagnostics
	data.Roles, d = types.SetValueFrom(ctx, types.StringType, user.Roles)
	diags.Append(d...)

	if user.AllAppsVisible {
		data.VisibleApps = types.SetNull(types.StringType)
	} else {
		data.VisibleApps, d = types.SetValueFrom(ctx, types.StringType, user.VisibleAppIDs)
		diags.Append(d...)
	}

	data.AllAppsVisible = types.BoolValue(user.AllAppsVisible)
	data.ProvisioningAllowed = types.BoolValue(user.ProvisioningAllowed)
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

	r.populateState(ctx, &data, user, resp.Diagnostics)

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

	// Wait for the deletion to propagate before returning: Apple's backend
	// does not always immediately release the user's email address for
	// reuse, and Terraform can call Delete and Create back-to-back for the
	// same email (e.g. when an unaccepted invitation's attributes change,
	// forcing a replace).
	if err := waitForUserDeleted(ctx, func() (*users.User, error) {
		return r.client.GetUser(ctx, data.ID.ValueString())
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("User was deleted but did not disappear from the API in time: %s", err))
		return
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err == nil {
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
		return
	}

	if !strings.Contains(req.ID, "@") {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("%q is not a valid import ID. Provide either the user's UUID or email address.", req.ID),
		)
		return
	}

	user, err := r.client.FindUserByEmail(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find user, got error: %s", err))
		return
	}

	if user == nil {
		resp.Diagnostics.AddError(
			"User not found",
			fmt.Sprintf("No App Store Connect user with email %q was found.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), user.ID)...)
}
