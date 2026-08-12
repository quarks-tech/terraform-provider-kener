package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccIncidentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
