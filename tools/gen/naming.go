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
			prevLower := i > 0 && !(runes[i-1] >= 'A' && runes[i-1] <= 'Z')
			nextLower := i+1 < len(runes) && !(runes[i+1] >= 'A' && runes[i+1] <= 'Z')
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

// Stub main function. Task 9 will add the real implementation.
func main() {
}
