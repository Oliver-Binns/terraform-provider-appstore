// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/oliver-binns/appstore-go"
)

// Ensure AppStoreConnectProvider satisfies various provider interfaces.
var _ provider.Provider = &AppStoreConnectProvider{}
var _ provider.ProviderWithFunctions = &AppStoreConnectProvider{}
var _ provider.ProviderWithEphemeralResources = &AppStoreConnectProvider{}

// AppStoreConnectProvider defines the provider implementation.
type AppStoreConnectProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// AppStoreConnectProviderModel describes the provider data model.
type AppStoreConnectProviderModel struct {
	IssuerID   types.String `tfsdk:"issuer_id"`
	KeyID      types.String `tfsdk:"key_id"`
	PrivateKey types.String `tfsdk:"private_key"`
}

func (p *AppStoreConnectProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "appstoreconnect"
	resp.Version = p.version
}

func (p *AppStoreConnectProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with Apple Developer Program resources using the App Store Connect API.",
		Attributes: map[string]schema.Attribute{
			"issuer_id": schema.StringAttribute{
				MarkdownDescription: "The issuer ID of the App Store Connect API key.",
				Required:            true,
			},
			"key_id": schema.StringAttribute{
				MarkdownDescription: "The key ID of the App Store Connect API key.",
				Required:            true,
			},
			"private_key": schema.StringAttribute{
				MarkdownDescription: "The private key of the App Store Connect API key.",
				Required:            true,
			},
		},
	}
}

func (p *AppStoreConnectProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data AppStoreConnectProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	client := appstore.AppStoreClient(
		data.KeyID.ValueString(),
		data.IssuerID.ValueString(),
		data.PrivateKey.ValueString(),
	)

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *AppStoreConnectProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
	}
}

func (p *AppStoreConnectProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *AppStoreConnectProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *AppStoreConnectProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AppStoreConnectProvider{
			version: version,
		}
	}
}
