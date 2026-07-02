package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &enablePreviewModeAction{}
	_ action.ActionWithConfigure = &enablePreviewModeAction{}
)

func NewEnablePreviewModeAction() action.Action {
	return &enablePreviewModeAction{}
}

type enablePreviewModeAction struct {
	client *matomo.Client
}

type previewModeActionModel struct {
	ContainerID types.String `tfsdk:"container_id"`
}

func (a *enablePreviewModeAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_enable_preview_mode"
}

func (a *enablePreviewModeAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables preview mode for a Tag Manager container's current draft version.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
		},
	}
}

func (a *enablePreviewModeAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *enablePreviewModeAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config previewModeActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{Message: "Enabling preview mode..."})
	if err := a.client.EnablePreviewMode(ctx, siteID, idContainer); err != nil {
		resp.Diagnostics.AddError("Error enabling Matomo Tag Manager preview mode", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Preview mode enabled."})
}
