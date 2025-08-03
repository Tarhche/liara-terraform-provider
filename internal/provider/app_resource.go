package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/tarhche/liara-terraform-provider/openapi/clients/paas"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppResource{}
var _ resource.ResourceWithImportState = &AppResource{}

func NewAppResource() resource.Resource {
	return &AppResource{}
}

// AppResource defines the resource implementation.
type AppResource struct {
	client paas.ClientInterface
}

// AppResourceModel describes the resource data model.
type AppResourceModel struct {
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

func (r *AppResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *AppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "App resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

func (r *AppResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = paasClient
}

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.CreateApp(ctx, paas.CreateAppJSONRequestBody{
		Name:   data.Name.ValueStringPointer(),
		PlanID: data.PlanID.ValueStringPointer(),
		//BundlePlanID:           data.BundlePlanID.ValueStringPointer(),
		Platform:               data.Platform.ValueStringPointer(),
		ReadOnlyRootFilesystem: data.ReadOnlyRootFilesystem.ValueBoolPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("App creation failed", fmt.Sprintf("Unable to create app, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			resp.Diagnostics.AddError("reading response payload failed", err.Error())

			return
		}

		resp.Diagnostics.AddError("App creation failed", fmt.Sprintf("Unable to create app, got error: %s", string(body)))
		return
	}

	tflog.Trace(ctx, "created an app resource")

	if data.TurnOff.ValueBool() {
		r.turnOff(ctx, &data, &resp.Diagnostics)
	}

	if data.RollingUpdate.ValueBool() {
		r.rollingUpdate(ctx, &data, &resp.Diagnostics)
	}

	if !data.Envs.IsNull() {
		r.updateEnvs(ctx, &data, &resp.Diagnostics)
	}

	if data.EnableStaticIP.ValueBool() {
		r.enableStaticIP(ctx, &data, &resp.Diagnostics)
	}

	if data.DisableDefaultSubDomain.ValueBool() {
		r.disableDefaultSubdomain(ctx, &data, &resp.Diagnostics)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.GetAppByName(ctx, data.Name.ValueString())
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

	tflog.Trace(ctx, "read app resource")

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AppResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.ChangePlan(ctx, data.Name.ValueString(), paas.ChangePlanJSONRequestBody{
		PlanID: data.PlanID.String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("App creation failed", fmt.Sprintf("Unable to create app, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			resp.Diagnostics.AddError("reading update response payload failed", err.Error())

			return
		}

		resp.Diagnostics.AddError("App creation failed", fmt.Sprintf("Unable to create app, got error: %s", string(body)))
		return
	}

	if data.TurnOff.ValueBool() {
		r.turnOff(ctx, &data, &resp.Diagnostics)
	}

	if data.RollingUpdate.ValueBool() {
		r.rollingUpdate(ctx, &data, &resp.Diagnostics)
	}

	if !data.Envs.IsNull() {
		r.updateEnvs(ctx, &data, &resp.Diagnostics)
	}

	if data.EnableStaticIP.ValueBool() {
		r.enableStaticIP(ctx, &data, &resp.Diagnostics)
	}

	if data.DisableDefaultSubDomain.ValueBool() {
		r.disableDefaultSubdomain(ctx, &data, &resp.Diagnostics)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.DeleteAppByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Deleting app failed", fmt.Sprintf("Unable to delete app, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			resp.Diagnostics.AddError("reading delete response payload failed", err.Error())

			return
		}

		resp.Diagnostics.AddError("Deleting app failed", fmt.Sprintf("Unable to delete app, got error: %s", string(body)))

		return
	}

	tflog.Trace(ctx, "deleted the app resource")
}

func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *AppResource) turnOff(ctx context.Context, data *AppResourceModel, diagnostics *diag.Diagnostics) {
	response, err := r.client.TurnApp(ctx, data.Name.ValueString(), paas.TurnAppJSONRequestBody{})
	if err != nil {
		diagnostics.AddError("Turning off the app failed", fmt.Sprintf("Unable to turn off the app, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			diagnostics.AddError("reading turn-off response payload failed", err.Error())

			return
		}

		diagnostics.AddError("Turning off the app failed", fmt.Sprintf("Unable to turn off the app, got error: %s", string(body)))

		return
	}

	tflog.Trace(ctx, "turned off the app")
}

func (r *AppResource) rollingUpdate(ctx context.Context, data *AppResourceModel, diagnostics *diag.Diagnostics) {
	switchMap := map[bool]string{
		true:  "enable",
		false: "disable",
	}

	response, err := r.client.ZeroDowntime(ctx, data.Name.ValueString(), switchMap[data.RollingUpdate.ValueBool()])
	if err != nil {
		diagnostics.AddError("Updating rolling-update configuration failed", fmt.Sprintf("Unable to update rolling-update configuration, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			diagnostics.AddError("reading rolling-update response payload failed", err.Error())

			return
		}

		diagnostics.AddError("Updating rolling-update configuration failed", fmt.Sprintf("Unable to update rolling-update configuration, got error: %s", string(body)))

		return
	}

	tflog.Trace(ctx, "updated rolling-update configuration")
}

func (r *AppResource) updateEnvs(ctx context.Context, data *AppResourceModel, diagnostics *diag.Diagnostics) {
	payload := paas.UpdateEnvsJSONRequestBody{
		Project: data.Name.ValueStringPointer(),
	}

	if err := data.Envs.ElementsAs(ctx, &payload.Variables, false); err != nil {
		diagnostics.Append(err...)

		return
	}

	response, err := r.client.UpdateEnvs(ctx, payload)
	if err != nil {
		diagnostics.AddError("Updating envs failed", fmt.Sprintf("Unable to update envs, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			diagnostics.AddError("reading envs response payload failed", err.Error())

			return
		}

		diagnostics.AddError("Updating envs failed", fmt.Sprintf("Unable to update envs, got error: %s", string(body)))
	}
}

func (r *AppResource) enableStaticIP(ctx context.Context, data *AppResourceModel, diagnostics *diag.Diagnostics) {
	switchMap := map[bool]string{
		true:  "enable",
		false: "disable",
	}

	response, err := r.client.IpStatic(ctx, data.Name.ValueString(), switchMap[data.EnableStaticIP.ValueBool()])
	if err != nil {
		diagnostics.AddError("Enabling static ip failed", fmt.Sprintf("Unable to enable static ip, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			diagnostics.AddError("reading static ip response payload failed", err.Error())

			return
		}

		diagnostics.AddError("Enabling static ip failed", fmt.Sprintf("Unable to enable static ip, got error: %s", string(body)))

		return
	}

	tflog.Trace(ctx, "enabled static ip")
}

func (r *AppResource) disableDefaultSubdomain(ctx context.Context, data *AppResourceModel, diagnostics *diag.Diagnostics) {
	switchMap := map[bool]string{
		true:  "enable",
		false: "disable",
	}

	response, err := r.client.DefaultSubdomain(ctx, data.Name.ValueString(), switchMap[!data.DisableDefaultSubDomain.ValueBool()])
	if err != nil {
		diagnostics.AddError("Disabling default subdomain failed", fmt.Sprintf("Unable to disable default subdomain, got error: %s", err))
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			diagnostics.AddError("reading default subdomain response payload failed", err.Error())

			return
		}

		diagnostics.AddError("Disabling default subdomain failed", fmt.Sprintf("Unable to disable default subdomain, got error: %s", string(body)))

		return
	}

	tflog.Trace(ctx, "disabled default subdomain")
}
