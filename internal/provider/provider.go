package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	providerName             string = "liara"
	defaultAPIEndpoint              = "https://api.iran.liara.ir"
	defaultWebsocketEndpoint        = "wss://api.iran.liara.ir"
	defaultTimeout           int64  = 30
)

// Ensure LiaraProvider satisfies various provider interfaces.
var _ provider.Provider = &LiaraProvider{}
var _ provider.ProviderWithFunctions = &LiaraProvider{}
var _ provider.ProviderWithEphemeralResources = &LiaraProvider{}

// LiaraProvider defines the provider implementation.
type LiaraProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// LiaraClient keeps the client configuration for data sources and resources.
type LiaraProviderData struct {
	APIEndpoint       string
	WebsocketEndpoint string
	AccessToken       string
	Timeout           time.Duration
	HTTPClient        *http.Client
}

// LiaraProviderModel describes the provider data model.
type LiaraProviderModel struct {
	APIEndpoint       types.String `tfsdk:"api_endpoint"`
	WebsocketEndpoint types.String `tfsdk:"websocket_endpoint"`
	AccessToken       types.String `tfsdk:"access_token"`
	Timeout           types.Int64  `tfsdk:"timeout"`
}

func (p *LiaraProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = providerName
	resp.Version = p.version
}

func (p *LiaraProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_endpoint": schema.StringAttribute{
				MarkdownDescription: "Liara API endpoint",
				Optional:            true,
			},
			"websocket_endpoint": schema.StringAttribute{
				MarkdownDescription: "Liara Websocket endpoint",
				Optional:            true,
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "Liara access token",
				Required:            true,
				Sensitive:           true,
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "Liara API timeout in seconds (default: 30)",
				Optional:            true,
			},
		},
	}
}

func (p *LiaraProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data LiaraProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if data.APIEndpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_endpoint"),
			"Unknown Liara API Endpoint",
			"The provider cannot create the Liara API client as there is an unknown configuration value for the Liara API endpoint. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the LIARA_API_ENDPOINT environment variable.",
		)
	}

	if data.WebsocketEndpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("websocket_endpoint"),
			"Unknown Liara Websocket Endpoint",
			"The provider cannot create the Liara API client as there is an unknown configuration value for the Liara Websocket endpoint. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the LIARA_WEBSOCKET_ENDPOINT environment variable.",
		)
	}

	if data.AccessToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Unknown Liara Access Token",
			"The provider cannot create the Liara API client as there is an unknown configuration value for the Liara Access Token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the LIARA_ACCESS_TOKEN environment variable.",
		)
	}

	if data.Timeout.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("timeout"),
			"Unknown Liara Timeout",
			"The provider cannot create the Liara API client as there is an unknown configuration value for the Liara Timeout. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the LIARA_TIMEOUT environment variable.",
		)
	}

	// 1. load defaults
	apiEndpoint := defaultAPIEndpoint
	websocketEndpoint := defaultWebsocketEndpoint
	timeout := defaultTimeout
	accessToken := ""

	// 2. override with ENV variables if set
	env_apiEndpoint := os.Getenv("LIARA_API_ENDPOINT")
	env_websocketEndpoint := os.Getenv("LIARA_WEBSOCKET_ENDPOINT")
	env_timeout := os.Getenv("LIARA_TIMEOUT")
	env_accessToken := os.Getenv("LIARA_ACCESS_TOKEN")

	if len(env_apiEndpoint) > 0 {
		apiEndpoint = env_apiEndpoint
	}

	if len(env_websocketEndpoint) > 0 {
		websocketEndpoint = env_websocketEndpoint
	}

	if len(env_timeout) > 0 {
		timeoutInt, err := strconv.ParseInt(env_timeout, 10, 64)
		if err != nil {
			resp.Diagnostics.AddError("Invalid timeout value", fmt.Sprintf("Invalid timeout value: %s", err))
			return
		}
		timeout = timeoutInt
	}

	if len(env_accessToken) > 0 {
		accessToken = env_accessToken
	}

	// 3. override with Terraform configs if set
	if !data.APIEndpoint.IsNull() {
		apiEndpoint = data.APIEndpoint.ValueString()
	}

	if !data.WebsocketEndpoint.IsNull() {
		websocketEndpoint = data.WebsocketEndpoint.ValueString()
	}

	if !data.Timeout.IsNull() {
		timeout = data.Timeout.ValueInt64()
	}

	if !data.AccessToken.IsNull() {
		accessToken = data.AccessToken.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if len(apiEndpoint) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_endpoint"),
			"Missing Liara API Endpoint",
			"The provider cannot create the Liara API client as there is a missing or empty value for the Liara API endpoint. "+
				"Set the api_endpoint value in the configuration or use the LIARA_API_ENDPOINT environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if len(websocketEndpoint) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("websocket_endpoint"),
			"Missing Liara Websocket Endpoint",
			"The provider cannot create the Liara API client as there is a missing or empty value for the Liara Websocket endpoint. "+
				"Set the websocket_endpoint value in the configuration or use the LIARA_WEBSOCKET_ENDPOINT environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if len(accessToken) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Missing Liara Access Token",
			"The provider cannot create the Liara API client as there is a missing or empty value for the Liara Access Token. "+
				"Set the access_token value in the configuration or use the LIARA_ACCESS_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// client configuration for data sources and resources
	providerData := &LiaraProviderData{
		APIEndpoint:       apiEndpoint,
		WebsocketEndpoint: websocketEndpoint,
		AccessToken:       accessToken,
		HTTPClient:        &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *LiaraProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAppResource,
	}
}

func (p *LiaraProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		// no ephemeral resources, at least for now!
	}
}

func (p *LiaraProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAppDataSource,
	}
}

func (p *LiaraProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		// no functions, at least for now!
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LiaraProvider{
			version: version,
		}
	}
}
