package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

func TestAccIncidentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIncidentDestroy,
		Steps: []resource.TestStep{
			// Create incident (+ attached monitor) and a comment.
			{
				Config: testAccIncidentConfig("TF Acc Incident", 1700000000, "INVESTIGATING"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("kener_incident.test", "id"),
					resource.TestCheckResourceAttr("kener_incident.test", "title", "TF Acc Incident"),
					resource.TestCheckResourceAttr("kener_incident.test", "start_date_time", "1700000000"),
					resource.TestCheckResourceAttr("kener_incident.test", "monitors.#", "1"),
					resource.TestCheckResourceAttr("kener_incident.test", "monitors.0.monitor_tag", "tf-acc-inc-mon"),
					resource.TestCheckResourceAttr("kener_incident.test", "monitors.0.impact", "DEGRADED"),
					resource.TestCheckResourceAttrSet("kener_incident.test", "state"),
					resource.TestCheckResourceAttrSet("kener_incident.test", "url"),
					resource.TestCheckResourceAttrSet("kener_incident_comment.test", "id"),
					resource.TestCheckResourceAttr("kener_incident_comment.test", "state", "INVESTIGATING"),
					resource.TestCheckResourceAttrSet("kener_incident_comment.test", "timestamp"),
				),
			},
			// Import the incident by id.
			{
				ResourceName:      "kener_incident.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Kener stores timestamps minute-aligned, so the imported value
				// may differ from the configured one by up to a minute.
				ImportStateVerifyIgnore: []string{"start_date_time"},
			},
			// Import the comment by "incident_id:comment_id".
			{
				ResourceName:      "kener_incident_comment.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["kener_incident_comment.test"]
					if !ok {
						return "", fmt.Errorf("comment resource not found in state")
					}
					return rs.Primary.Attributes["incident_id"] + ":" + rs.Primary.Attributes["id"], nil
				},
			},
			// Update incident (add end time) and resolve the comment; also read
			// it back through the data source and verify the comments list.
			{
				Config: testAccIncidentConfigResolved("TF Acc Incident Updated", 1700000000, 1700003600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_incident.test", "title", "TF Acc Incident Updated"),
					resource.TestCheckResourceAttr("kener_incident.test", "end_date_time", "1700003600"),
					resource.TestCheckResourceAttr("kener_incident_comment.test", "state", "RESOLVED"),
					// Data source exposes the incident and its comments.
					resource.TestCheckResourceAttr("data.kener_incident.test", "title", "TF Acc Incident Updated"),
					resource.TestCheckResourceAttr("data.kener_incident.test", "monitors.#", "1"),
					resource.TestCheckResourceAttr("data.kener_incident.test", "comments.#", "1"),
					resource.TestCheckResourceAttr("data.kener_incident.test", "comments.0.state", "RESOLVED"),
				),
			},
		},
	})
}

// TestAccIncidentResource_multipleMonitors guards the C1 fix: an incident with
// several monitors must preserve the configured order in state. Before the fix
// the server echo was written back unconditionally, which could reorder a known
// plan value and fail with "inconsistent result after apply".
func TestAccIncidentResource_multipleMonitors(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIncidentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccIncidentConfigMultiMonitors("TF Acc Multi Incident", 1700000000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.#", "3"),
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.0.monitor_tag", "tf-acc-inc-m1"),
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.0.impact", "DOWN"),
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.1.monitor_tag", "tf-acc-inc-m2"),
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.1.impact", "DEGRADED"),
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.2.monitor_tag", "tf-acc-inc-m3"),
					resource.TestCheckResourceAttr("kener_incident.multi", "monitors.2.impact", "MAINTENANCE"),
				),
			},
		},
	})
}

func testAccIncidentConfigMultiMonitors(title string, start int64) string {
	return fmt.Sprintf(`
resource "kener_monitor" "m1" {
  tag          = "tf-acc-inc-m1"
  name         = "M1"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_monitor" "m2" {
  tag          = "tf-acc-inc-m2"
  name         = "M2"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_monitor" "m3" {
  tag          = "tf-acc-inc-m3"
  name         = "M3"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_incident" "multi" {
  title           = %[1]q
  start_date_time = %[2]d
  monitors = [
    { monitor_tag = kener_monitor.m1.tag },
    { monitor_tag = kener_monitor.m2.tag, impact = "DEGRADED" },
    { monitor_tag = kener_monitor.m3.tag, impact = "MAINTENANCE" },
  ]
}
`, title, start)
}

// testAccCheckIncidentDestroy verifies every kener_incident in state is gone
// from the server after destroy.
func testAccCheckIncidentDestroy(s *terraform.State) error {
	c, err := testAccClient()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kener_incident" {
			continue
		}
		id := rs.Primary.Attributes["id"]
		_, err := c.GetIncident(context.Background(), id)
		if err == nil {
			return fmt.Errorf("incident %q still exists after destroy", id)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("unexpected error checking incident %q: %w", id, err)
		}
	}
	return nil
}

func testAccIncidentConfig(title string, start int64, commentState string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "inc_mon" {
  tag          = "tf-acc-inc-mon"
  name         = "TF Acc Incident Monitor"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_incident" "test" {
  title           = %[1]q
  start_date_time = %[2]d
  monitors = [
    { monitor_tag = kener_monitor.inc_mon.tag, impact = "DEGRADED" },
  ]
}

resource "kener_incident_comment" "test" {
  incident_id = kener_incident.test.id
  comment     = "Investigating the issue"
  state       = %[3]q
}
`, title, start, commentState)
}

func testAccIncidentConfigResolved(title string, start, end int64) string {
	return fmt.Sprintf(`
resource "kener_monitor" "inc_mon" {
  tag          = "tf-acc-inc-mon"
  name         = "TF Acc Incident Monitor"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_incident" "test" {
  title           = %[1]q
  start_date_time = %[2]d
  end_date_time   = %[3]d
  monitors = [
    { monitor_tag = kener_monitor.inc_mon.tag, impact = "DEGRADED" },
  ]
}

resource "kener_incident_comment" "test" {
  incident_id = kener_incident.test.id
  comment     = "Issue resolved"
  state       = "RESOLVED"
}

data "kener_incident" "test" {
  id         = kener_incident.test.id
  depends_on = [kener_incident_comment.test]
}
`, title, start, end)
}
