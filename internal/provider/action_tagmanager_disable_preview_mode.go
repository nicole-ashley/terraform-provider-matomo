package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &disablePreviewModeAction{}
	_ action.ActionWithConfigure = &disablePreviewModeAction{}
)

func NewDisablePreviewModeAction() action.Action {
	return &disablePreviewModeAction{}
}

type disablePreviewModeAction struct {
	client *matomo.Client
}

func (a *disablePreviewModeAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_disable_preview_mode"
}

func (a *disablePreviewModeAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Disables preview mode for a Tag Manager container's current draft version.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
		},
	}
}

func (a *disablePreviewModeAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *disablePreviewModeAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
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

	resp.SendProgress(action.InvokeProgressEvent{Message: "Disabling preview mode..."})
	if err := a.client.DisablePreviewMode(ctx, siteID, idContainer); err != nil {
		resp.Diagnostics.AddError("Error disabling Matomo Tag Manager preview mode", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Preview mode disabled."})
}
