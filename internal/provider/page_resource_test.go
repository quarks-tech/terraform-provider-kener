package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPageResource(t *testing.T) {
	const path = "tf-acc-page"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create + Read (with one attached monitor).
			{
				Config: testAccPageConfig(path, "TF Acc Page", "All Systems Operational"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_page.test", "page_path", path),
					resource.TestCheckResourceAttr("kener_page.test", "page_title", "TF Acc Page"),
					resource.TestCheckResourceAttr("kener_page.test", "page_header", "All Systems Operational"),
					resource.TestCheckResourceAttrSet("kener_page.test", "id"),
					resource.TestCheckResourceAttr("kener_page.test", "monitors.#", "1"),
					resource.TestCheckResourceAttr("kener_page.test", "monitors.0", "tf-acc-page-mon"),
				),
			},
			// Import. page_settings is not recovered verbatim (server merges
			// defaults), so it is ignored on import verification.
			{
				ResourceName:            "kener_page.test",
				ImportState:             true,
				ImportStateId:           path,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"page_settings"},
			},
			// Update in place: change header/subheader and drop the monitor.
			{
				Config: testAccPageConfigNoMonitors(path, "TF Acc Page", "Renamed Header"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_page.test", "page_header", "Renamed Header"),
					resource.TestCheckResourceAttr("kener_page.test", "page_subheader", "updated"),
				),
			},
		},
	})
}

func testAccPageConfig(path, title, header string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "page_mon" {
  tag          = "tf-acc-page-mon"
  name         = "TF Acc Page Monitor"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_page" "test" {
  page_path   = %[1]q
  page_title  = %[2]q
  page_header = %[3]q
  monitors    = [kener_monitor.page_mon.tag]
}
`, path, title, header)
}

func testAccPageConfigNoMonitors(path, title, header string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "page_mon" {
  tag          = "tf-acc-page-mon"
  name         = "TF Acc Page Monitor"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_page" "test" {
  page_path      = %[1]q
  page_title     = %[2]q
  page_header    = %[3]q
  page_subheader = "updated"
  monitors       = []
}
`, path, title, header)
}
