package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

func TestAccMaintenanceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckMaintenanceDestroy,
		Steps: []resource.TestStep{
			// Create with a non-default status (exercises the create-then-patch
			// path) and two monitors (guards C1: configured order is preserved).
			{
				Config: testAccMaintenanceConfig("TF Acc Maintenance", "FREQ=WEEKLY;BYDAY=SU", 3600, "INACTIVE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("kener_maintenance.test", "id"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "title", "TF Acc Maintenance"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "rrule", "FREQ=WEEKLY;BYDAY=SU"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "duration_seconds", "3600"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "status", "INACTIVE"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "monitors.#", "2"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "monitors.0.monitor_tag", "tf-acc-maint-m1"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "monitors.0.impact", "MAINTENANCE"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "monitors.1.monitor_tag", "tf-acc-maint-m2"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "monitors.1.impact", "DEGRADED"),
					resource.TestCheckResourceAttrSet("kener_maintenance.test", "url"),
				),
			},
			// Import by id (start_date_time is minute-aligned server-side).
			{
				ResourceName:            "kener_maintenance.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"start_date_time"},
			},
			// Update: change title/duration/rrule and re-enable.
			{
				Config: testAccMaintenanceConfig("TF Acc Maintenance Updated", "FREQ=DAILY;INTERVAL=2", 1800, "ACTIVE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_maintenance.test", "title", "TF Acc Maintenance Updated"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "rrule", "FREQ=DAILY;INTERVAL=2"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "duration_seconds", "1800"),
					resource.TestCheckResourceAttr("kener_maintenance.test", "status", "ACTIVE"),
				),
			},
		},
	})
}

func testAccMaintenanceConfig(title, rrule string, duration int64, status string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "maint_m1" {
  tag          = "tf-acc-maint-m1"
  name         = "TF Acc Maintenance Monitor 1"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_monitor" "maint_m2" {
  tag          = "tf-acc-maint-m2"
  name         = "TF Acc Maintenance Monitor 2"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_maintenance" "test" {
  title            = %[1]q
  start_date_time  = 1700000400
  rrule            = %[2]q
  duration_seconds = %[3]d
  status           = %[4]q
  monitors = [
    { monitor_tag = kener_monitor.maint_m1.tag },
    { monitor_tag = kener_monitor.maint_m2.tag, impact = "DEGRADED" },
  ]
}
`, title, rrule, duration, status)
}

// testAccCheckMaintenanceDestroy verifies every kener_maintenance in state is
// gone from the server after destroy.
func testAccCheckMaintenanceDestroy(s *terraform.State) error {
	c, err := testAccClient()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kener_maintenance" {
			continue
		}
		id := rs.Primary.Attributes["id"]
		_, err := c.GetMaintenance(context.Background(), id)
		if err == nil {
			return fmt.Errorf("maintenance %q still exists after destroy", id)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("unexpected error checking maintenance %q: %w", id, err)
		}
	}
	return nil
}
