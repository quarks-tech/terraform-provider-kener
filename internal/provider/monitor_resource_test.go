package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMonitorResource(t *testing.T) {
	const tag = "tf-acc-monitor"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create + Read.
			{
				Config: testAccMonitorConfigBasic(tag, "TF Acc Monitor"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_monitor.test", "tag", tag),
					resource.TestCheckResourceAttr("kener_monitor.test", "name", "TF Acc Monitor"),
					resource.TestCheckResourceAttr("kener_monitor.test", "monitor_type", "API"),
					// Server-defaulted computed values.
					resource.TestCheckResourceAttr("kener_monitor.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("kener_monitor.test", "default_status", "UP"),
					resource.TestCheckResourceAttr("kener_monitor.test", "is_hidden", "false"),
					resource.TestCheckResourceAttr("kener_monitor.test", "confirmation_threshold", "1"),
					resource.TestCheckResourceAttrSet("kener_monitor.test", "id"),
					// type_data is stored verbatim as configured.
					resource.TestCheckResourceAttr("kener_monitor.test", "type_data", `{"url":"https://example.com"}`),
				),
			},
			// ImportState. type_data / monitor_settings_json cannot be recovered
			// from the server verbatim (Kener merges server-side defaults), so
			// they are ignored on import verification.
			{
				ResourceName:            "kener_monitor.test",
				ImportState:             true,
				ImportStateId:           tag,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"type_data", "monitor_settings_json"},
			},
			// Update in place.
			{
				Config: testAccMonitorConfigUpdated(tag, "TF Acc Monitor Renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_monitor.test", "name", "TF Acc Monitor Renamed"),
					resource.TestCheckResourceAttr("kener_monitor.test", "description", "managed by terraform"),
					resource.TestCheckResourceAttr("kener_monitor.test", "is_hidden", "true"),
					resource.TestCheckResourceAttr("kener_monitor.test", "confirmation_threshold", "3"),
					resource.TestCheckResourceAttr("kener_monitor.test", "status", "INACTIVE"),
				),
			},
		},
	})
}

func testAccMonitorConfigBasic(tag, name string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "test" {
  tag          = %[1]q
  name         = %[2]q
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}
`, tag, name)
}

func testAccMonitorConfigUpdated(tag, name string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "test" {
  tag                    = %[1]q
  name                   = %[2]q
  description            = "managed by terraform"
  monitor_type           = "API"
  status                 = "INACTIVE"
  is_hidden              = true
  confirmation_threshold = 3
  type_data              = jsonencode({ url = "https://example.com" })
}
`, tag, name)
}
