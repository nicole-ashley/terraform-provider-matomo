package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &createContainerVersionAction{}
	_ action.ActionWithConfigure = &createContainerVersionAction{}
)

func NewCreateContainerVersionAction() action.Action {
	return &createContainerVersionAction{}
}

type createContainerVersionAction struct {
	client *matomo.Client
}

type createContainerVersionActionModel struct {
	ContainerID types.String `tfsdk:"container_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (a *createContainerVersionAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_create_container_version"
}

func (a *createContainerVersionAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Snapshots a Tag Manager container's current draft into a new, named, immutable version. Does not publish it - see matomo_tagmanager_publish_container_version for that.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The new version's name. Matomo requires 1-50 characters.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The new version's description.",
			},
		},
	}
}

func (a *createContainerVersionAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*matomo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", "expected *matomo.Client")
		return
	}
	a.client = client
}

func (a *createContainerVersionAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config createContainerVersionActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	description := config.Description.ValueString()

	resp.SendProgress(action.InvokeProgressEvent{Message: "Creating container version..."})
	versionID, err := a.client.CreateContainerVersion(ctx, siteID, idContainer, config.Name.ValueString(), description)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Matomo Tag Manager container version", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Created container version."})
	_ = versionID // the created version's id cannot be exposed as an action output (see design spec §2) - it is only ever visible via the progress messages above.
}
