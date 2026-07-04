// tools/gen/naming.go
package main

import "strings"

// Slug converts a Matomo template id (e.g. "CustomHtml") into the
// lowercase form used in generated resource names
// (matomo_tagmanager_tag_customhtml).
func Slug(id string) string {
	return strings.ToLower(id)
}

// CamelToSnake converts a Matomo camelCase parameter name (e.g.
// "htmlPosition") into the snake_case form used for generated Terraform
// attribute names ("html_position"). Consecutive uppercase letters (as in
// an acronym like "URLPath") are treated as a single word boundary rather
// than one boundary per letter, so "URLPath" becomes "url_path" not
// "u_r_l_path".
func CamelToSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper {
			prevLower := i > 0 && (runes[i-1] < 'A' || runes[i-1] > 'Z')
			nextLower := i+1 < len(runes) && (runes[i+1] < 'A' || runes[i+1] > 'Z')
			if i > 0 && (prevLower || nextLower) {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ExportedName converts a Matomo camelCase parameter name into an
// exported Go identifier ("htmlPosition" -> "HtmlPosition"), for use as a
// generated model struct field name.
func ExportedName(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	return string(r)
}

// SnakeToPascal converts a snake_case word (as used for a ListOfObjects
// row key, e.g. "consent_type") into an exported Go identifier
// ("ConsentType"), for use as a generated row struct field name. Unlike
// ExportedName (which only capitalizes an already-camelCase Matomo
// parameter name's first letter), this also removes underscores and
// capitalizes the letter after each one.
func SnakeToPascal(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(s, "_") {
		b.WriteString(ExportedName(part))
	}
	return b.String()
}

// singularIrregulars holds the plural->singular mappings that plain
// trailing-"s" stripping gets wrong. Only "data"->"datum" is known to
// occur in a Matomo UI_CONTROL_MULTI_TUPLE parameter name today
// (MatomoConfiguration's customData) - add further entries here if a
// future field needs one, rather than teaching Singularize new grammar
// rules for a single word.
var singularIrregulars = map[string]string{
	"data": "datum",
}

// Singularize converts a plural snake_case word (as used in a generated
// Terraform attribute name, e.g. "custom_dimensions") into its singular
// form ("custom_dimension"), for naming the nested block a
// UI_CONTROL_MULTI_TUPLE list-of-objects parameter's rows live under -
// this project's convention names the block after one row, not the list
// (see docs/superpowers/plans/2026-07-03-list-of-object-parameters.md).
// Only the last underscore-separated word is singularized, so a
// multi-word attribute name like "custom_dimensions" correctly becomes
// "custom_dimension", not some other word in the name getting mangled.
func Singularize(s string) string {
	i := strings.LastIndex(s, "_")
	prefix, last := "", s
	if i >= 0 {
		prefix, last = s[:i+1], s[i+1:]
	}
	if singular, ok := singularIrregulars[last]; ok {
		return prefix + singular
	}
	return prefix + strings.TrimSuffix(last, "s")
}
