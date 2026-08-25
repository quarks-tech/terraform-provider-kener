package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMonitorSeedResource(t *testing.T) {
	const tag = "tf-acc-monitor-seed"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// The monitor is torn down by its own CheckDestroy; the seed leaves data
		// behind server-side by design, so it needs no destroy check.
		CheckDestroy: testAccCheckMonitorDestroy,
		Steps: []resource.TestStep{
			// Create: seed with defaults (90 days, UP).
			{
				Config: testAccMonitorSeedConfig(tag, 90),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_monitor_seed.test", "monitor_tag", tag),
					resource.TestCheckResourceAttr("kener_monitor_seed.test", "window_days", "90"),
					resource.TestCheckResourceAttr("kener_monitor_seed.test", "status", "UP"),
					resource.TestCheckResourceAttr("kener_monitor_seed.test", "latency", "0"),
					resource.TestCheckResourceAttr("kener_monitor_seed.test", "deviation", "0"),
					resource.TestCheckResourceAttrSet("kener_monitor_seed.test", "id"),
					resource.TestCheckResourceAttrSet("kener_monitor_seed.test", "start_ts"),
					resource.TestCheckResourceAttrSet("kener_monitor_seed.test", "end_ts"),
				),
			},
			// Drift guard: re-applying the same config must produce an empty plan
			// (Read is a no-op, computed values are pinned).
			{
				Config:   testAccMonitorSeedConfig(tag, 90),
				PlanOnly: true,
			},
			// Changing the window forces replacement and re-resolves the range.
			{
				Config: testAccMonitorSeedConfig(tag, 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_monitor_seed.test", "window_days", "30"),
				),
			},
		},
	})
}

func testAccMonitorSeedConfig(tag string, windowDays int) string {
	return fmt.Sprintf(`
resource "kener_monitor" "test" {
  tag          = %[1]q
  name         = "TF Acc Monitor Seed"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_monitor_seed" "test" {
  monitor_tag = kener_monitor.test.tag
  window_days = %[2]d
}
`, tag, windowDays)
}
