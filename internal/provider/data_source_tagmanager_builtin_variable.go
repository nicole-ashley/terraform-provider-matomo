package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &tagManagerBuiltinVariableDataSource{}

func NewTagManagerBuiltinVariableDataSource() datasource.DataSource {
	return &tagManagerBuiltinVariableDataSource{}
}

// tagManagerBuiltinVariableDataSource has no configuration and makes no API
// call - every attribute value is a fixed constant, confirmed against
// Matomo's own Template/Variable/PreConfigured/*.php source (see this
// resource's implementation plan for how each was confirmed). This exists
// purely so these identifiers are referenceable (and therefore
// editor-autocompletable) instead of typed as bare strings - it is not
// validation and does not represent every variable name Matomo or a
// third-party plugin might accept.
type tagManagerBuiltinVariableDataSource struct{}

// builtinVariableIDs maps each Terraform-facing snake_case attribute name to
// its real Matomo Type ID, confirmed per this resource's implementation
// plan Task 1.
var builtinVariableIDs = map[string]string{
	"browser_language":             "BrowserLanguage",
	"click_button":                 "ClickButton",
	"click_classes":                "ClickClasses",
	"click_destination_url":        "ClickDestinationUrl",
	"click_element":                "ClickElement",
	"click_id":                     "ClickId",
	"click_node_name":              "ClickNodeName",
	"click_text":                   "ClickText",
	"container_id":                 "ContainerId",
	"container_revision":           "ContainerRevision",
	"container_version":            "ContainerVersion",
	"dns_lookup_time":              "DnsLookupTime",
	"environment":                  "Environment",
	"error_line":                   "ErrorLine",
	"error_message":                "ErrorMessage",
	"error_url":                    "ErrorUrl",
	"first_directory":              "FirstDirectory",
	"form_classes":                 "FormClasses",
	"form_destination":             "FormDestination",
	"form_element":                 "FormElement",
	"form_id":                      "FormId",
	"form_name":                    "FormName",
	"history_hash_new_path":        "HistoryHashNewPath",
	"history_hash_new_search":      "HistoryHashNewSearch",
	"history_hash_new_url":         "HistoryHashNewUrl",
	"history_hash_new":             "HistoryHashNew",
	"history_hash_old_path":        "HistoryHashOldPath",
	"history_hash_old_search":      "HistoryHashOldSearch",
	"history_hash_old_url":         "HistoryHashOldUrl",
	"history_hash_old":             "HistoryHashOld",
	"history_source":               "HistorySource",
	"iso_date":                     "IsoDate",
	"local_date":                   "LocalDate",
	"local_hour":                   "LocalHour",
	"local_time":                   "LocalTime",
	"page_hash":                    "PageHash",
	"page_hostname":                "PageHostname",
	"page_load_time_total":         "PageLoadTimeTotal",
	"page_origin":                  "PageOrigin",
	"page_path":                    "PagePath",
	"page_render_time":             "PageRenderTime",
	"page_title":                   "PageTitle",
	"page_url":                     "PageUrl",
	"preview_mode":                 "PreviewMode",
	"random_number":                "RandomNumber",
	"referrer":                     "Referrer",
	"screen_height":                "ScreenHeight",
	"screen_height_available":      "ScreenHeightAvailable",
	"screen_width":                 "ScreenWidth",
	"screen_width_available":       "ScreenWidthAvailable",
	"scroll_horizontal_percentage": "ScrollHorizontalPercentage",
	"scroll_left_pixel":            "ScrollLeftPixel",
	"scroll_source":                "ScrollSource",
	"scroll_top_pixel":             "ScrollTopPixel",
	"scroll_vertical_percentage":   "ScrollVerticalPercentage",
	"seo_canonical_url":            "SeoCanonicalUrl",
	"seo_num_h1":                   "SeoNumH1",
	"seo_num_h2":                   "SeoNumH2",
	"user_agent":                   "UserAgent",
	"utc_date":                     "UtcDate",
	"visible_element_classes":      "VisibleElementClasses",
	"visible_element_id":           "VisibleElementId",
	"visible_element_node_name":    "VisibleElementNodeName",
	"visible_element_text":         "VisibleElementText",
	"visible_element_url":          "VisibleElementUrl",
	"weekday":                      "Weekday",
}

func (d *tagManagerBuiltinVariableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tagmanager_builtin_variable"
}

func (d *tagManagerBuiltinVariableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Always \"builtin\" - this data source has no configuration, so its id is a fixed constant.",
		},
	}
	for name, id := range builtinVariableIDs {
		attrs[name] = schema.StringAttribute{
			Computed:    true,
			Description: "Matomo's built-in \"" + id + "\" trigger-condition variable identifier.",
		}
	}
	resp.Schema = schema.Schema{
		Description: "Matomo's built-in, always-available trigger-condition variables (e.g. PagePath, ClickId), exposed as named attributes so they can be referenced instead of typed as bare strings. This is a discoverability aid only - it is not an exhaustive or validated list of every value condition.variable accepts (third-party plugins can contribute more, and any user-defined matomo_tagmanager_variable* resource is referenceable via a {{Name}} macro regardless of this data source).",
		Attributes:  attrs,
	}
}

func (d *tagManagerBuiltinVariableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	attrTypes := map[string]attr.Type{"id": types.StringType}
	attrValues := map[string]attr.Value{"id": types.StringValue("builtin")}
	for name, id := range builtinVariableIDs {
		attrTypes[name] = types.StringType
		attrValues[name] = types.StringValue(id)
	}

	obj, diags := types.ObjectValue(attrTypes, attrValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
