package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestStrPtr(t *testing.T) {
	if got := strPtr(types.StringNull()); got != nil {
		t.Errorf("strPtr(null) = %v, want nil", got)
	}
	if got := strPtr(types.StringUnknown()); got != nil {
		t.Errorf("strPtr(unknown) = %v, want nil", got)
	}
	if got := strPtr(types.StringValue("x")); got == nil || *got != "x" {
		t.Errorf("strPtr(\"x\") = %v, want \"x\"", got)
	}
}

func TestStrValue(t *testing.T) {
	if got := strValue(nil); !got.IsNull() {
		t.Errorf("strValue(nil) = %v, want null", got)
	}
	s := "hello"
	if got := strValue(&s); got.ValueString() != "hello" {
		t.Errorf("strValue(&\"hello\") = %q, want \"hello\"", got.ValueString())
	}
	empty := ""
	if got := strValue(&empty); got.IsNull() || got.ValueString() != "" {
		t.Errorf("strValue(&\"\") = %v, want non-null empty string", got)
	}
}

func TestInt64Value(t *testing.T) {
	if got := int64Value(nil); !got.IsNull() {
		t.Errorf("int64Value(nil) = %v, want null", got)
	}
	n := int64(42)
	if got := int64Value(&n); got.ValueInt64() != 42 {
		t.Errorf("int64Value(&42) = %d, want 42", got.ValueInt64())
	}
}

func TestBoolToYesNo(t *testing.T) {
	tests := []struct {
		name string
		in   types.Bool
		want *string
	}{
		{"null", types.BoolNull(), nil},
		{"unknown", types.BoolUnknown(), nil},
		{"true", types.BoolValue(true), ptr(yesValue)},
		{"false", types.BoolValue(false), ptr(noValue)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boolToYesNo(tt.in)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("boolToYesNo(%v) = %q, want nil", tt.in, *got)
			case tt.want != nil && (got == nil || *got != *tt.want):
				t.Errorf("boolToYesNo(%v) = %v, want %q", tt.in, got, *tt.want)
			}
		})
	}
}

func TestYesNoToBool(t *testing.T) {
	yes, no, other := yesValue, noValue, "maybe"
	tests := []struct {
		name string
		in   *string
		want bool
	}{
		{"nil", nil, false},
		{"yes", &yes, true},
		{"no", &no, false},
		{"unrecognised", &other, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := yesNoToBool(tt.in); got.ValueBool() != tt.want {
				t.Errorf("yesNoToBool(%v) = %v, want %v", tt.in, got.ValueBool(), tt.want)
			}
		})
	}
}

func TestJSONValue(t *testing.T) {
	tests := []struct {
		name     string
		in       []byte
		wantNull bool
		wantStr  string
	}{
		{"nil", nil, true, ""},
		{"empty", []byte{}, true, ""},
		{"literal null", []byte("null"), true, ""},
		{"object", []byte(`{"a":1}`), false, `{"a":1}`},
		{"string", []byte(`"title"`), false, `"title"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonValue(tt.in)
			if got.IsNull() != tt.wantNull {
				t.Fatalf("jsonValue(%q).IsNull() = %v, want %v", tt.in, got.IsNull(), tt.wantNull)
			}
			if !tt.wantNull && got.ValueString() != tt.wantStr {
				t.Errorf("jsonValue(%q) = %q, want %q", tt.in, got.ValueString(), tt.wantStr)
			}
		})
	}
}

func TestJSONRaw(t *testing.T) {
	if got := jsonRaw(jsontypes.NewNormalizedNull()); got != nil {
		t.Errorf("jsonRaw(null) = %q, want nil", got)
	}
	if got := jsonRaw(jsontypes.NewNormalizedUnknown()); got != nil {
		t.Errorf("jsonRaw(unknown) = %q, want nil", got)
	}
	if got := jsonRaw(jsontypes.NewNormalizedValue(`{"a":1}`)); string(got) != `{"a":1}` {
		t.Errorf("jsonRaw(`{\"a\":1}`) = %q, want `{\"a\":1}`", got)
	}
}

// jsonValue/jsonRaw round-trip: encoding then decoding a JSON object preserves it.
func TestJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"url":"https://example.com"}`)
	v := jsonValue(raw)
	if got := jsonRaw(v); string(got) != string(raw) {
		t.Errorf("round-trip = %q, want %q", got, raw)
	}
}

func ptr(s string) *string { return &s }
