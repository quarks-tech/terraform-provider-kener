package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	fwschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// monitorTypeAttribute pulls the monitor_type attribute out of the resource
// schema so the tests below exercise the validators the provider actually
// installs, not a locally rebuilt copy of them.
func monitorTypeAttribute(t *testing.T) fwschema.StringAttribute {
	t.Helper()

	var resp fwresource.SchemaResponse
	NewMonitorResource().Schema(context.Background(), fwresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned diagnostics: %v", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["monitor_type"].(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("monitor_type is %T, want schema.StringAttribute", resp.Schema.Attributes["monitor_type"])
	}
	return attr
}

func validateMonitorType(t *testing.T, attr fwschema.StringAttribute, value string) validator.StringResponse {
	t.Helper()

	req := validator.StringRequest{
		Path:        path.Root("monitor_type"),
		ConfigValue: types.StringValue(value),
	}
	var resp validator.StringResponse
	for _, v := range attr.Validators {
		v.ValidateString(context.Background(), req, &resp)
	}
	return resp
}

// Every type Kener itself accepts must survive plan-time validation. PROMETHEUS
// (Kener 4.1.3) and DOCKER (Kener 4.1.5) are the reason this test exists: the
// provider's list lagged upstream and made them unconfigurable.
func TestMonitorTypeValidatorAcceptsEveryKenerType(t *testing.T) {
	t.Parallel()

	attr := monitorTypeAttribute(t)
	for _, monitorType := range monitorTypes {
		t.Run(monitorType, func(t *testing.T) {
			if resp := validateMonitorType(t, attr, monitorType); resp.Diagnostics.HasError() {
				t.Errorf("monitor_type %q rejected: %v", monitorType, resp.Diagnostics)
			}
		})
	}
}

func TestMonitorTypeValidatorRejectsUnknownType(t *testing.T) {
	t.Parallel()

	attr := monitorTypeAttribute(t)
	if resp := validateMonitorType(t, attr, "NOT_A_MONITOR_TYPE"); !resp.Diagnostics.HasError() {
		t.Error("monitor_type \"NOT_A_MONITOR_TYPE\" accepted, want a validation error")
	}
}

// The documented list and the validated list are maintained by hand in the same
// file, so they can drift apart and leave a usable type looking unsupported.
func TestMonitorTypeDescriptionListsEveryType(t *testing.T) {
	t.Parallel()

	attr := monitorTypeAttribute(t)
	for _, monitorType := range monitorTypes {
		if !strings.Contains(attr.MarkdownDescription, "`"+monitorType+"`") {
			t.Errorf("monitor_type description does not mention %q: %s", monitorType, attr.MarkdownDescription)
		}
	}
}
