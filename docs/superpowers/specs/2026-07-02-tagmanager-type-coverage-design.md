# terraform-provider-matomo — full Tag Manager type coverage design

Status: approved design, pre-implementation.

## 1. Goal and scope

`matomo_tagmanager_tag`, `matomo_tagmanager_trigger`, and
`matomo_tagmanager_variable` are generic resources: a `type` string plus
repeated `parameter { name, value }` blocks, passed through to Matomo
largely uninterpreted. The provider code itself doesn't special-case any
particular type, so structurally it should work with any built-in Matomo
Tag Manager template without code changes.

But the previous acceptance-testing slice (PR #5) spent many CI
round-trips discovering that individual types carry their own wire-format
quirks that only show up against real Matomo — wrong required parameter
key names, auto-filled defaults that must be declared to avoid perpetual
diffs, response/request key casing mismatches. That slice only exercises
one representative type per resource: `CustomHtml` (tag), `PageView`
(trigger), `Constant` (variable). Every other built-in type is
structurally untested.

This is a public module. We can't rely on noticing every such bug by hand
before a user hits it in a type we never exercised. This slice adds
acceptance-test coverage for every creatable built-in Tag/Trigger/Variable
template Matomo's core `tag-manager` plugin ships — roughly 30-40 types
across the three resources.

**Creatable** excludes Matomo's ~70 read-only `PreConfigured` variables
(`PagePath`, `ClickElement`, `ClickText`, etc., under
`Template/Variable/PreConfigured/`). Those are referenced by name (e.g. as
a trigger condition's `actual` value, as already used in this project's
existing `withConditions` trigger test) — there is no `addContainerVariable`
call for them, so there's no create/read cycle to test.

## 2. Out of scope

- Import/disappears coverage per new type. That logic (composite ID
  parsing, not-found detection, idempotent delete) is generic and already
  proven for one representative type per resource. Every new type gets
  create + read-back coverage only.
- The ~70 read-only `PreConfigured` variables (see above — not created via
  the resource, nothing to test).
- Non-core marketplace tag templates (e.g. paid/third-party Tag Manager
  templates) — only what ships in Matomo's own `tag-manager` plugin.
- Any typed/per-type Go resource codegen, schema changes, or new resource
  types. This slice is test coverage only; the generic resource design is
  unchanged. (Typed-resource codegen remains future work per the original
  foundation spec.)
- Consolidating existing tests' site/container creation pattern. Every
  acceptance test — old and new — continues to create its own
  `matomo_site` + `matomo_tagmanager_container`, matching current
  behavior. CI runtime cost from more types is accepted as a trade-off for
  consistency and simplicity.

## 3. Test architecture

Per resource (tag, trigger, variable), a new table-driven test function
covers every type that fits the generic "flat string parameters" shape:

```go
func TestAccTagManagerVariableResource_types(t *testing.T) {
    testAccPreCheck(t)

    cases := []struct {
        Type       string
        Parameters map[string]string
    }{
        {Type: "Cookie", Parameters: map[string]string{"cookieName": "acc_test_cookie"}},
        {Type: "UrlParameter", Parameters: map[string]string{"urlParameterName": "utm_source"}},
        // ... one entry per researched type
    }

    for _, tc := range cases {
        t.Run(tc.Type, func(t *testing.T) {
            // build a full HCL config from tc.Parameters (its own
            // matomo_site + matomo_tagmanager_container + the resource
            // under test), apply, assert each parameter round-trips
            // through Read() unchanged
        })
    }
}
```

Each subtest builds and applies its **own** complete config — its own
`matomo_site` + `matomo_tagmanager_container` + the resource under test —
matching today's one-site-and-container-per-test pattern exactly, just
organized as a loop over a table in the Go code rather than N
copy-pasted functions. The table-driven structure is a code-maintenance
device (add a row, not a function); it does not change how many
site/container pairs get created at runtime. Each subtest asserts via
`TestCheckResourceAttr` that the declared parameters are present after
apply (proving the create call was accepted and Read() didn't silently
drop or rename anything).

Subtests are individually visible in `go test -v` output as
`TestAccTagManagerVariableResource_types/Cookie`, so a failure names the
exact type without needing to read assertion messages.

**Dedicated tests for structural outliers:** any type whose parameters
aren't flat strings — confirmed so far: `MatomoConfiguration` variable,
which references custom dimensions by name rather than taking a plain
string value — gets its own standalone test function (following the
existing `_basic`/`withConditions`-style naming), not forced into the
table.

**Existing tests unchanged:** `CustomHtml`/`PageView`/`Constant`'s current
full `_basic`/`_import`/`_disappears`/(`_multipleParameters`/
`_withConditions`) coverage stays exactly as-is. The new table-driven
tests are additive.

## 4. Research process

Before any test code is written, each resource gets a research task that
produces a definitive table: every creatable type's exact identifier
string, its required/notable parameter keys (via that template's
`makeSetting()`/`getParameters()` definition in Matomo's `tag-manager`
source — the same method already used to confirm `Constant`→
`constantValue` and `CustomHtml`→`customHtml`/`htmlPosition`), and whether
it fits the generic table or needs a dedicated test.

This research is a plan task, not something left to implementer subagents
to improvise per-type — guessing at this scale (30-40 types) would risk
the same iterate-via-CI cost the last slice paid for 3 types, multiplied
out.

**Known starting point** (from this session, needs re-verification against
the pinned Matomo version but not re-discovery): Variable templates under
`Template/Variable/*.php` (excluding `PreConfigured/`): `Constant`,
`Cookie`, `CustomJsFunction`, `DataLayer`, `DomElement`,
`EtrackerConfiguration`, `JavaScript`, `MatomoConfiguration`,
`MetaContent`, `ReferrerUrl`, `TimeSinceLoad`, `UrlParameter`, `Url` — 13
types, `Constant` already covered by the existing test.

**Not yet enumerated:** Tag templates and Trigger templates. Both are
fresh research for the implementation plan (locate `Template/Tag/*.php`
and `Template/Trigger/*.php` in Matomo's `tag-manager` source, following
the same pattern used to find `Template/Variable/`).

## 5. Delivery and phasing

One spec (this document), one implementation plan, one PR, executed via
`subagent-driven-development` matching the prior two slices. The plan
sequences three phases:

1. **Variable types** — catalog already known (§4), smallest remaining
   research burden. Research task confirms/updates the 13-type list and
   each one's parameters, then implementation tasks add the table-driven
   test plus a dedicated `MatomoConfiguration` test.
2. **Tag types** — research task enumerates `Template/Tag/*.php` and each
   type's parameters, then implementation tasks add the table-driven test
   plus any dedicated tests for outliers found during research.
3. **Trigger types** — same shape as tags: research task enumerates
   `Template/Trigger/*.php`, then implementation tasks.

No new CI infrastructure: these are new test functions in the existing
`internal/provider/resource_tagmanager_{tag,trigger,variable}_acc_test.go`
files, running against the same `acceptance.yml` / Docker Compose Matomo
fixture already in place.

## 6. Risks and open questions

- The exact count of Tag and Trigger templates is unknown until the
  research tasks run; this could shift the "30-40 types" estimate in
  either direction. Not a blocker — the phased plan structure absorbs
  this naturally (each phase's research task produces its own accurate
  count).
- Some types may turn out to have required parameters that themselves
  need real infrastructure to set meaningfully (e.g. a type requiring a
  valid external URL, or credentials for a third-party service). Where
  that happens, the research task should note it and the implementation
  task should use a syntactically-valid-but-fake value (matching how
  `default_value = "n/a"` and similar placeholder values are already used
  in existing tests) rather than skipping the type — the goal is
  confirming Matomo *accepts* the wire format, not exercising the
  integration's real-world behavior.
