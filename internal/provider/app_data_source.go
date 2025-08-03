package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tarhche/liara-terraform-provider/openapi/clients/paas"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AppDataSource{}

func NewAppDataSource() datasource.DataSource {
	return &AppDataSource{}
}

// AppDataSource defines the data source implementation.
type AppDataSource struct {
	client paas.ClientInterface
}

// AppDataSourceModel describes the data source data model.
type AppDataSourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	PlanID                 types.String `tfsdk:"plan_id"`
	BundlePlanID           types.String `tfsdk:"bundle_plan_id"`
	Platform               types.String `tfsdk:"platform"`
	ReadOnlyRootFilesystem types.Bool   `tfsdk:"read_only_root_filesystem"`
	NetworkName            types.String `tfsdk:"network_name"`

	RollingUpdate           types.Bool   `tfsdk:"rolling_update"`
	TurnOff                 types.Bool   `tfsdk:"turn_off"`
	Envs                    types.Map    `tfsdk:"envs"`
	StaticIP                types.String `tfsdk:"static_ip"`
	EnableStaticIP          types.Bool   `tfsdk:"enable_static_ip"`
	DisableDefaultSubDomain types.Bool   `tfsdk:"disable_default_subdomain"`
}

func (d *AppDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (d *AppDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "App data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "identifier",
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "name",
				Required:            true,
			},
			"plan_id": schema.StringAttribute{
				MarkdownDescription: "plan id",
				Required:            true,
			},
			"bundle_plan_id": schema.StringAttribute{
				MarkdownDescription: "bundle plan id",
				Optional:            true,
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "platform",
				Required:            true,
			},
			"read_only_root_filesystem": schema.BoolAttribute{
				MarkdownDescription: "read only root filesystem",
				Required:            true,
			},
			"network_name": schema.StringAttribute{
				MarkdownDescription: "network name",
				Optional:            true,
			},
			"rolling_update": schema.BoolAttribute{
				MarkdownDescription: "rolling update",
				Optional:            true,
			},
			"turn_off": schema.BoolAttribute{
				MarkdownDescription: "is the app should be turned off or not (true for turn off, false for turning on)",
				Optional:            true,
			},
			"envs": schema.MapAttribute{
				MarkdownDescription: "environment variables",
				Optional:            true,
				ElementType:         types.StringType,
				Sensitive:           true,
			},
			"static_ip": schema.StringAttribute{
				MarkdownDescription: "static ip",
				Optional:            true,
			},
			"enable_static_ip": schema.BoolAttribute{
				MarkdownDescription: "enable static ip",
				Optional:            true,
			},
			"disable_default_subdomain": schema.BoolAttribute{
				MarkdownDescription: "disable default subdomain",
				Optional:            true,
			},
		},
	}
}

func (d *AppDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*LiaraProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	paasClient, err := paas.NewClient(
		providerData.APIEndpoint,
		paas.WithHTTPClient(providerData.HTTPClient),
		paas.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", providerData.AccessToken))
			return nil
		}),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create PAAS client",
			fmt.Sprintf("Expected paas.ClientInterface, got: %T. Please report this issue to the provider developers.", err),
		)

		return
	}

	d.client = paasClient
}

func (d *AppDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AppDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := d.client.GetAppByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Reading App info failed", fmt.Sprintf("Unable to read app info, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			resp.Diagnostics.AddError("reading response payload failed", err.Error())

			return
		}

		resp.Diagnostics.AddError("Reading App info failed", fmt.Sprintf("Unable to read app info, got error: %s", string(body)))
		return
	}

	responseModel := struct {
		Project struct {
			ID                     string `json:"_id"`
			ProjectID              string `json:"project_id"`
			Type                   string `json:"type"`
			Status                 string `json:"status"`
			DefaultSubdomain       bool   `json:"defaultSubdomain"`
			ReadOnlyRootFilesystem bool   `json:"readOnlyRootFilesystem"`
			ZeroDowntime           bool   `json:"zeroDowntime"`
			Scale                  int    `json:"scale"`
			Envs                   []struct {
				Key       string `json:"key"`
				Value     string `json:"value"`
				Encrypted bool   `json:"encrypted"`
			} `json:"envs"`
			PlanID       string `json:"planID"`
			BundlePlanID string `json:"bundlePlanID"`
			Network      struct {
				ID   string `json:"_id"`
				Name string `json:"name"`
			} `json:"network"`
			FixedIPStatus string `json:"fixedIPStatus"`
			CreatedAt     string `json:"created_at"`
			Node          struct {
				ID string `json:"_id"`
				IP string `json:"IP"`
			} `json:"node"`
			HourlyPrice       int  `json:"hourlyPrice"`
			IsDeployed        bool `json:"isDeployed"`
			ReservedDiskSpace int  `json:"reservedDiskSpace"`
		} `json:"project"`
	}{}

	if err := json.NewDecoder(response.Body).Decode(&responseModel); err != nil {
		resp.Diagnostics.AddError("Decoding read response failed", fmt.Sprintf("Unable to decode read response, got error: %s", err))
		return
	}

	envs := make(map[string]attr.Value)
	for _, env := range responseModel.Project.Envs {
		envs[env.Key] = types.StringValue(env.Value)
	}

	data.ID = types.StringValue(responseModel.Project.ID)
	data.Name = types.StringValue(responseModel.Project.ID)
	data.PlanID = types.StringValue(responseModel.Project.PlanID)
	data.BundlePlanID = types.StringValue(responseModel.Project.BundlePlanID)
	data.Platform = types.StringValue(responseModel.Project.Type)
	data.ReadOnlyRootFilesystem = types.BoolValue(responseModel.Project.ReadOnlyRootFilesystem)
	data.NetworkName = types.StringValue(responseModel.Project.Network.Name)
	data.RollingUpdate = types.BoolValue(responseModel.Project.ZeroDowntime)
	data.TurnOff = types.BoolValue(responseModel.Project.Scale == 0)
	data.Envs = types.MapValueMust(types.StringType, envs)

	data.EnableStaticIP = types.BoolValue(len(responseModel.Project.Node.IP) > 0)
	if data.EnableStaticIP.ValueBool() {
		data.StaticIP = types.StringValue(responseModel.Project.Node.IP)
	}

	data.DisableDefaultSubDomain = types.BoolValue(!responseModel.Project.DefaultSubdomain)

	tflog.Trace(ctx, "read app data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
