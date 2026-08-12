// Package provider implements the Terraform provider for Kener, including the
// provider configuration and each resource and data source.
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// yesValue / noValue are how Kener stores boolean-ish monitor fields.
const (
	yesValue = "YES"
	noValue  = "NO"
)

// strPtr converts a Terraform string to a *string, returning nil for null or
// unknown values (so the client omits or nulls the field as configured).
func strPtr(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	return &v
}

// strValue converts a *string from the API into a Terraform string, mapping nil
// to null.
func strValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// int64Ptr converts a Terraform int64 to a *int64, nil for null/unknown.
func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	x := v.ValueInt64()
	return &x
}

// int64Value converts a *int64 from the API into a Terraform int64, nil -> null.
func int64Value(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

// boolToYesNo maps a Terraform bool to Kener's "YES"/"NO" string, nil for
// null/unknown so the server default/existing value applies.
func boolToYesNo(b types.Bool) *string {
	if b.IsNull() || b.IsUnknown() {
		return nil
	}
	if b.ValueBool() {
		v := yesValue
		return &v
	}
	v := noValue
	return &v
}

// yesNoToBool maps Kener's "YES"/"NO" string back to a Terraform bool. A nil or
// unrecognised value maps to false (the server default).
func yesNoToBool(p *string) types.Bool {
	if p == nil {
		return types.BoolValue(false)
	}
	return types.BoolValue(*p == yesValue)
}

// jsonValue converts raw JSON bytes from the API into a normalized JSON string
// value, mapping nil/empty/"null" to a Terraform null.
func jsonValue(raw []byte) jsontypes.Normalized {
	if len(raw) == 0 || string(raw) == "null" {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(raw))
}

// jsonRaw converts a normalized JSON string value into raw JSON bytes for the
// API, returning nil for null/unknown.
func jsonRaw(v jsontypes.Normalized) []byte {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return []byte(v.ValueString())
}

// stringSlice converts a Terraform list of strings to []string, returning nil
// for null/unknown so the caller can omit the field.
func stringSlice(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	out := make([]string, 0, len(l.Elements()))
	diags := l.ElementsAs(ctx, &out, false)
	return out, diags
}

// stringList converts a []string from the API into a Terraform list of strings.
func stringList(ctx context.Context, s []string) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(ctx, types.StringType, s)
}
