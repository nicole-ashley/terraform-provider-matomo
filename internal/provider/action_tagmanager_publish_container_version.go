package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nicole-ashley/terraform-provider-matomo/internal/matomo"
)

var (
	_ action.Action              = &publishContainerVersionAction{}
	_ action.ActionWithConfigure = &publishContainerVersionAction{}
)

func NewPublishContainerVersionAction() action.Action {
	return &publishContainerVersionAction{}
}

type publishContainerVersionAction struct {
	client *matomo.Client
}

type publishContainerVersionActionModel struct {
	ContainerID types.String `tfsdk:"container_id"`
	Environment types.String `tfsdk:"environment"`
}

func (a *publishContainerVersionAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_publish_container_version"
}

func (a *publishContainerVersionAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Publishes a Tag Manager container's current draft to an environment. Internally snapshots the draft into a new version, then publishes that snapshot in the same invocation - there is no separate version input, this action always publishes whatever the draft currently holds.",
		Attributes: map[string]schema.Attribute{
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "The container's id (matomo_tagmanager_container.x.id).",
			},
			"environment": schema.StringAttribute{
				Required:    true,
				Description: "The environment to publish to, e.g. \"live\". See the matomo_tagmanager_environments data source for valid values on a given instance.",
			},
		},
	}
}

func (a *publishContainerVersionAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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

func (a *publishContainerVersionAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config publishContainerVersionActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	siteID, idContainer, err := parseContainerID(config.ContainerID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid container_id", err.Error())
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{Message: "Snapshotting current draft..."})
	versionName := "terraform-release-" + time.Now().UTC().Format(time.RFC3339)
	versionID, err := a.client.CreateContainerVersion(ctx, siteID, idContainer, versionName, "")
	if err != nil {
		resp.Diagnostics.AddError("Error snapshotting Matomo Tag Manager container draft", err.Error())
		return
	}

	environment := config.Environment.ValueString()
	resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Publishing version %d to environment %q...", versionID, environment)})
	if err := a.client.PublishContainerVersion(ctx, siteID, idContainer, versionID, environment); err != nil {
		resp.Diagnostics.AddError("Error publishing Matomo Tag Manager container version", err.Error())
		return
	}
	resp.SendProgress(action.InvokeProgressEvent{Message: "Published."})
}
