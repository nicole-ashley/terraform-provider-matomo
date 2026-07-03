# Tag Manager Provider Gaps (Trigger Conditions + List-of-Object Parameters) - Design

## 1. Goal and scope

Fix real gaps in the v0.0.1 provider surface, found during an actual migration and
verified against Matomo's live PHP source (`matomo-org/tag-manager`, `5.x-dev`
branch) rather than assumed:

1. **Typed trigger conditions.** Every generated `matomo_tagmanager_trigger_<type>`
   resource silently drops `conditions` - core functionality, since Matomo
   triggers routinely require conditions to fire correctly, not an edge case.
2. **List-of-object parameters.** Six fields across five typed resources
   (`custom_dimensions`/`custom_data` on `variable_matomoconfiguration`,
   `custom_dimensions` on `tag_matomo` and `variable_etrackerconfiguration`,
   `event_parameters` on `tag_googleanalytics4event`, `consent_types` on
   `tag_googleconsentmodev2`) are modeled as flat `list(string)`, but Matomo's
   real wire format needs a list of named-field objects per row
   (`parameters[name][0][key]=...`) - the flat form is structurally incapable
   of representing this, not merely awkward.
3. **Built-in trigger-condition variables.** Renaming the confusing `actual`
   attribute to `variable`, adding validation against Matomo's real catalog of
   68 built-in "pre-configured" variables (`PagePath`, `PageHostname`, `ClickId`,
   etc. - confirmed via `Template/Variable/PreConfigured/`'s directory
   listing), and a new data source so these can be referenced instead of typed
   as bare strings.

A third originally-reported gap (`ClickDataAttribute.lookup_table`) was
investigated and found to be a non-issue: Matomo's real
`ClickDataAttributeVariable.php` declares exactly one parameter
(`dataAttribute`), and Matomo has no dedicated "Lookup Table" variable type at
all. Out of scope - nothing to fix.

Also out of scope, deferred to a separate future design: splitting
`matomo_site.urls` into `main`/`aliases` attributes, and an acceptance test
confirming whether changing the main URL post-creation is actually supported
by Matomo (this is unrelated to Tag Manager and was raised as a tangent during
this design's brainstorm).

`tag_googleconsentmodev2.consent_action` was also found to be `TYPE_ARRAY` in
Matomo's source, but with `UI_CONTROL_SINGLE_SELECT` (a single value from a
fixed set), not `UI_CONTROL_MULTI_TUPLE` like the six fields above - a smaller,
different oddity, not fixed by this design. Worth a follow-up look, not a
blocker here.

## 2. Typed trigger conditions

Every generated trigger model gains conditions support via the *shared*
typed-trigger runtime and `tools/gen`'s trigger template - not per-type, so
every current and future generated trigger type gets it automatically.

- `typedTriggerCommon` (`internal/provider/typed_trigger_resource.go`) gains a
  `Condition []conditionModel` field, using the exact shape the generic
  trigger resource already has (`resource_tagmanager_trigger.go`):
  ```go
  type conditionModel struct {
      Comparison string `tfsdk:"comparison"`
      Actual     string `tfsdk:"variable"` // see section 4 for the rename
      Value      string `tfsdk:"value"`
  }
  ```
- The shared runtime's `Create`/`Update` (`typed_trigger_resource.go:88-92,
  181-185`) each gain one line: `Conditions: conditionsToParams(common.Condition)`,
  reusing the *existing* `conditionsToParams`/`conditionsFromAPI` helpers the
  generic trigger resource already defines - no new condition-serialization
  logic, just wiring the typed layer to the same functions.
- `Read` populates `common.Condition` from `conditionsFromAPI(trig.Conditions)`,
  mirroring the generic resource's `Read`.

HCL becomes identical across typed and generic triggers:

```hcl
resource "matomo_tagmanager_trigger_customevent" "example" {
  container_id = matomo_tagmanager_container.main.id
  name         = "Add to Cart"
  event_name   = "add_to_cart"

  condition {
    comparison = "equals"
    variable   = "PagePath"
    value      = "/cart"
  }
}
```

## 3. Renaming `actual` to `variable`

The generic trigger resource's existing `condition` block (`actual` attribute)
is renamed to `variable` at the same time typed triggers gain conditions, so
both stay consistent. This is a breaking schema change to an already-shipped
attribute - acceptable now because no real tagged release has gone out yet
(the user's `0.0.1` release is still pending); it would need a deprecation
path after that point.

Rationale (from investigating Matomo's actual `WebContext.php` source): a
condition's `actual` field is resolved by loading a "Variable Template" for
it - built-in identifiers like `PagePath` and user-defined
`{{My Variable}}` references both resolve through the *same* mechanism.
`variable` names this accurately; `actual` is GTM-borrowed jargon that only
makes sense paired against "expected," and reads unclearly as a standalone
Terraform attribute name.

## 4. Built-in variable catalog and reference data source

Matomo ships 68 built-in "pre-configured" variables
(`Template/Variable/PreConfigured/*.php`) usable as a trigger condition's
`variable` value or as a `{{Name}}` macro anywhere a variable reference is
valid - confirmed to exist as a real, distinct catalog from the
user-instantiable variable types this provider already generates typed
resources for (they live in a separate subdirectory, and are never
user-creatable resources - they're always available, like a global constant).

**No validation is added against this catalog.** `variable` stays an
unvalidated string, same as `actual` is today. Reasoning (corrected during
spec review): a fixed `OneOf` list sourced only from Matomo core's own
repository would incorrectly reject two classes of legitimate value it
doesn't know about - a third-party plugin can contribute its own
pre-configured variables beyond Matomo core's 68, and `variable` can also
reference any user-created `matomo_tagmanager_variable*` resource by name via
`{{Name}}`, which is inherently open-ended and can never be enumerated ahead
of time. The catalog and its data source (below) exist purely so the *known*
built-in names are discoverable and referenceable without memorizing exact
casing - not as a completeness gate on what's valid.

**Investigation task (before finalizing the exact list):** the file-name-derived
candidate identifiers need confirming against live Matomo, for two things:
(a) a few files break the `*Variable.php` naming convention (e.g.
`ClickElement.php`, `ScreenHeight.php`), so their exact `getName()`-equivalent
string needs checking rather than assumed from the filename; (b) whether
these 68 appear in the same discovery API response
(`TagManager.getAvailableVariableTypesInContext`) our codegen already
consumes - if yes, this list can be auto-verified/regenerated by codegen
rather than hand-typed once and left to drift.

Full 68-file candidate list, derived from `Template/Variable/PreConfigured/`'s
directory listing (confirmed against `matomo-org/tag-manager`'s `5.x-dev`
branch), one candidate Type ID per file with the `Variable.php`/`.php` suffix
stripped:

```
BaseDataLayerVariable, BasePreConfiguredVariable (both likely abstract base
classes, not real selectable types - confirm and exclude if so), BrowserLanguage,
ClickButton, ClickClasses, ClickDestinationUrl, ClickElement, ClickId,
ClickNodeName, ClickText, ContainerId, ContainerRevision, ContainerVersion,
DnsLookupTime, Environment, ErrorLine, ErrorMessage, ErrorUrl, FirstDirectory,
FormClasses, FormDestination, FormElement, FormId, FormName,
HistoryHashNewPath, HistoryHashNewSearch, HistoryHashNewUrl, HistoryHashNew,
HistoryHashOldPath, HistoryHashOldSearch, HistoryHashOldUrl, HistoryHashOld,
HistorySource, IsoDate, LocalDate, LocalHour, LocalTime, PageHash,
PageHostname, PageLoadTimeTotal, PageOrigin, PagePath, PageRenderTime,
PageTitle, PageUrl, PreviewMode, RandomNumber, Referrer, ScreenHeight,
ScreenHeightAvailable, ScreenWidth, ScreenWidthAvailable,
ScrollHorizontalPercentage, ScrollLeftPixel, ScrollSource, ScrollTopPixel,
ScrollVerticalPercentage, SeoCanonicalUrl, SeoNumH1, SeoNumH2, UserAgent,
UtcDate, VisibleElementClasses, VisibleElementId, VisibleElementNodeName,
VisibleElementText, VisibleElementUrl, Weekday
```

`BaseDataLayerVariable` and `BasePreConfiguredVariable` are almost certainly
shared base classes (matching the naming convention of `BaseVariable.php` in
the parent directory, which is not itself a selectable variable type) rather
than real selectable types - the investigation task must confirm and exclude
them if so, leaving 66 real entries.

Once confirmed, a new data source, `matomo_tagmanager_builtin_variable` (final
name TBD in the plan), exposes each catalog entry as a named, `Computed`
string attribute (snake_case name -> the real PascalCase identifier), purely
so users get real editor autocomplete - Terraform's plugin protocol has no
enum concept at all, so a validator would be invisible to any static tooling
anyway (confirmed: autocomplete only works for known resource/attribute names
and cross-resource references, never validator-constrained string values). A
reference to a *known schema attribute*, by contrast, is exactly what editor
autocomplete already handles correctly:

```hcl
data "matomo_tagmanager_builtin_variable" "this" {}

resource "matomo_tagmanager_trigger_pageview" "example" {
  # ...
  condition {
    comparison = "equals"
    variable   = data.matomo_tagmanager_builtin_variable.this.page_path
    value      = "/checkout"
  }
}
```

`variable = data.matomo_tagmanager_builtin_variable.this.page_path` and
`variable = "PagePath"` are wire-identical - this is purely a discoverability
layer, not new capability, and not a completeness/correctness gate (see
above - it must not reject anything `variable` accepts today). Documentation
must show *both* forms (bare string literal, and the data-source reference),
since the verbosity tradeoff is a real, user-facing decision, not something
this design should make on the user's behalf.

## 5. List-of-object parameters

### 5.1 Core data model (`internal/matomo/formencoding.go`)

`ParamValue` gains a third shape alongside the existing `Scalar`/`List`:

```go
type ParamValue struct {
    Scalar        string
    List          []string
    ListOfObjects []map[string]string
}

func ListOfObjectsParam(rows []map[string]string) ParamValue { return ParamValue{ListOfObjects: rows} }
func (v ParamValue) IsListOfObjects() bool { return v.ListOfObjects != nil }
```

`addParamsMap` gains a case emitting `name[key][i][subkey]=value` for each row
of a `ListOfObjects` entry (mirroring the existing per-item loop for `List`).
`decodeParamValue` gains a case detecting a JSON array of objects (as opposed
to the existing array-of-strings case) and building `ListOfObjects` from it.

### 5.2 The `domains` special case

`variable_matomoconfiguration.domains` is `TYPE_ARRAY` +
`UI_CONTROL_MULTI_TUPLE` in Matomo's source too, but each "row" has exactly
one key (`domain`) - there is no pairing information a nested block would add
over a flat list, confirmed by Matomo's own PHP transform
(`MatomoConfigurationVariable.php`) filtering `$domain['domain']` uniformly
with no per-position special-casing. This field stays a plain `list(string)`
in schema/model/HCL; only its wire encoding changes, via a small
`wrapSingleKeyParam(key string, items []string) ParamValue` helper that builds
a `ListOfObjects` under the hood from a flat `[]string` - transparent to the
Terraform-facing side entirely.

### 5.3 Typed resource schema changes

Each of the 6 real list-of-object fields gets a real nested block with named
sub-attributes matching Matomo's actual `MultiPair` key names (verified
per-field against source, not a fixed convention - `customDimensions` is
`{index,value}`, `customData` is `{name,value}`, `eventParameters` is
`{parameter,value}`, `consentTypes` is `{consent_type,consent_state}`, renamed
per the user's preference to `{type,state}`):

```hcl
resource "matomo_tagmanager_variable_matomoconfiguration" "example" {
  # ...
  custom_dimension {
    index = matomo_custom_dimension.page_category.index
    value = "{{Page Category}}"
  }
  custom_datum {
    name  = "Environment"
    value = "production"
  }
  domains = ["*.example.com"]  # stays flat, see 5.2
}
```

Each affected typed resource's model, schema, `ToParams`, and `FromParams`
follow the same pattern already established for `condition{}` on the generic
trigger resource - a `[]rowModel` field, a `schema.ListNestedBlock`, and
row-shape-specific (not generic) conversion helpers.

### 5.4 Generic resource schema changes

`matomo_tagmanager_tag`/`trigger`/`variable` (which already share a single
`tagParameterModel` type, confirmed at
`resource_tagmanager_tag.go:35`/`resource_tagmanager_trigger.go:214`/
`resource_tagmanager_variable.go:185`) gain a new, similarly-shared block,
`parameter_list`, using nested `row`/`item` blocks (Option B from the
brainstorm - idiomatic nested blocks, matching this provider's existing
`condition{}`/`parameter{}` convention, over a `list(map(string))` attribute or
a `jsonencode()`-string, both rejected as less idiomatic):

```hcl
resource "matomo_tagmanager_variable" "example" {
  container_id = matomo_tagmanager_container.main.id
  type         = "MatomoConfiguration"
  name         = "example"

  parameter_list {
    name = "customDimensions"
    row {
      item { key = "index", value = matomo_custom_dimension.page_category.index }
      item { key = "value", value = "{{Page Category}}" }
    }
    row {
      item { key = "index", value = matomo_custom_dimension.user_type.index }
      item { key = "value", value = "{{User Type}}" }
    }
  }
}
```

```go
type parameterListModel struct {
    Name types.String        `tfsdk:"name"`
    Row  []parameterRowModel `tfsdk:"row"`
}
type parameterRowModel struct {
    Item []parameterItemModel `tfsdk:"item"`
}
type parameterItemModel struct {
    Key   types.String `tfsdk:"key"`
    Value types.String `tfsdk:"value"`
}
```

A new shared helper (alongside the existing `parametersToMap`) converts
`[]parameterListModel` into `ListOfObjects`-shaped `matomo.ParamsMap` entries;
the read-back direction converts a `ListOfObjects` `ParamValue` back into
`row`/`item` blocks, sorting each row's `item`s by `key` alphabetically for
deterministic state (Matomo's own response has no inherent key order).

### 5.5 Investigation task (before finalizing 5.3/5.4's implementation approach)

Write an acceptance test that calls Matomo's discovery API directly
(`TagManager.getAvailableVariableTypesInContext`) and inspects the *raw* JSON
for `MatomoConfigurationVariable`'s `customDimensions` parameter, to settle
whether `uiControl`/`uiControlAttributes` (and the `field1`/`field2` sub-key
names) are exposed generically. This determines whether `tools/gen` can be
extended to *auto-detect* `UI_CONTROL_MULTI_TUPLE` parameters and emit the
right nested block for any current or future field with this shape (the
strictly better outcome - no hand-curation, self-updating), or whether the 6
known fields need a hand-curated override table (similar to
`tools/gen/required.go`'s existing pattern) mapping `(kind, typeID, paramName)`
to its row shape. Either way, the *typed schema output* for the 6 known
fields is identical (section 5.3) - this investigation only affects whether
`tools/gen` gets a new general capability or a narrower override table.

## 6. Testing

- The section 5.5 investigation task itself is the first piece of testing
  work - it's expected to fail/reveal-information before any fix code exists,
  by design.
- New acceptance tests for each of the 6 typed list-of-object fields, actually
  submitting rows and confirming Matomo accepts them (the `has to be an array`
  rejection this design exists to fix), plus read-back verification.
- New acceptance tests for typed trigger conditions (mirroring the existing
  generic trigger's `TestAccTagManagerTriggerResource_withConditions`) on at
  least one generated trigger type.
- New acceptance/unit tests for the generic `parameter_list` block on at least
  one tag and one variable (shared code, but exercised via both resource
  kinds per this provider's existing convention).
- Unit tests for the new `ParamValue.ListOfObjects` wire encoding/decoding in
  `formencoding.go`, and for `wrapSingleKeyParam`.
- The new builtin-variable data source gets ordinary acceptance-test coverage
  once section 4's investigation confirms the final catalog.

## 7. Out of scope

- `ClickDataAttribute.lookup_table` - confirmed non-issue (section 1).
- `matomo_site.urls` -> `main`/`aliases` split - deferred, separate future
  design.
- `tag_googleconsentmodev2.consent_action`'s `UI_CONTROL_SINGLE_SELECT` oddity
  - noted, not fixed here.
