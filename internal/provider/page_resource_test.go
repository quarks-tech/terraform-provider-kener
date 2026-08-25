package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/quarks-tech/terraform-provider-kener/internal/client"
)

// TestPagePathPattern guards the page_path validator: ordinary lowercase slugs
// are accepted, the literal ~home is accepted, and near-misses are rejected.
func TestPagePathPattern(t *testing.T) {
	re := regexp.MustCompile(pagePathPattern)
	valid := []string{"~home", "status", "a", "my-page", "a_b-c", "page1"}
	invalid := []string{"~foo", "home~", "", "Foo", "~Home", "-abc", "abc-", "~", "a~b"}
	for _, s := range valid {
		if !re.MatchString(s) {
			t.Errorf("expected %q to be a valid page_path", s)
		}
	}
	for _, s := range invalid {
		if re.MatchString(s) {
			t.Errorf("expected %q to be an invalid page_path", s)
		}
	}
}

func TestAccPageResource(t *testing.T) {
	const path = "tf-acc-page"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPageDestroy,
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

// TestAccPageResource_multipleMonitors guards the C1 fix for page.monitors: a
// page with several monitors must preserve the configured order in state (the
// server echo could otherwise be reordered and trigger an inconsistent-result
// error on apply).
func TestAccPageResource_multipleMonitors(t *testing.T) {
	const path = "tf-acc-page-multi"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPageDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPageConfigMultiMonitors(path),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_page.multi", "monitors.#", "3"),
					// Order must match the configuration exactly.
					resource.TestCheckResourceAttr("kener_page.multi", "monitors.0", "tf-acc-page-m1"),
					resource.TestCheckResourceAttr("kener_page.multi", "monitors.1", "tf-acc-page-m2"),
					resource.TestCheckResourceAttr("kener_page.multi", "monitors.2", "tf-acc-page-m3"),
				),
			},
		},
	})
}

func testAccPageConfigMultiMonitors(path string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "m1" {
  tag          = "tf-acc-page-m1"
  name         = "M1"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_monitor" "m2" {
  tag          = "tf-acc-page-m2"
  name         = "M2"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_monitor" "m3" {
  tag          = "tf-acc-page-m3"
  name         = "M3"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_page" "multi" {
  page_path   = %[1]q
  page_title  = "TF Acc Multi Page"
  page_header = "Multi"
  monitors    = [kener_monitor.m1.tag, kener_monitor.m2.tag, kener_monitor.m3.tag]
}
`, path)
}

// TestAccPageResource_home exercises managing the built-in home page (~home):
// it is imported (never created), then updated in place, and a final plan must
// be empty (no drift). The home page must NOT be destroyed, so this case has no
// CheckDestroy.
func TestAccPageResource_home(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Import the pre-existing home page and persist it into state so the
			// following config-driven step updates it instead of creating it.
			{
				Config:             testAccPageConfigHome("TF Home", "Home Header"),
				ResourceName:       "kener_page.home",
				ImportState:        true,
				ImportStateId:      client.HomePageToken,
				ImportStatePersist: true,
			},
			// Update in place: apply title/header and attach one monitor.
			{
				Config: testAccPageConfigHome("TF Home", "Home Header"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_page.home", "page_path", client.HomePageToken),
					resource.TestCheckResourceAttr("kener_page.home", "page_title", "TF Home"),
					resource.TestCheckResourceAttr("kener_page.home", "page_header", "Home Header"),
					resource.TestCheckResourceAttr("kener_page.home", "monitors.#", "1"),
					resource.TestCheckResourceAttr("kener_page.home", "monitors.0", "tf-acc-home-mon"),
				),
			},
			// Drift guard: re-applying the same config must be a no-op.
			{
				Config:   testAccPageConfigHome("TF Home", "Home Header"),
				PlanOnly: true,
			},
		},
	})
}

func testAccPageConfigHome(title, header string) string {
	return fmt.Sprintf(`
resource "kener_monitor" "home_mon" {
  tag          = "tf-acc-home-mon"
  name         = "TF Acc Home Monitor"
  monitor_type = "API"
  type_data    = jsonencode({ url = "https://example.com" })
}

resource "kener_page" "home" {
  page_path   = "~home"
  page_title  = %[1]q
  page_header = %[2]q
  monitors    = [kener_monitor.home_mon.tag]
}
`, title, header)
}

// testAccCheckPageDestroy verifies every kener_page in state is gone from the
// server after destroy.
func testAccCheckPageDestroy(s *terraform.State) error {
	c, err := testAccClient()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kener_page" {
			continue
		}
		path := rs.Primary.Attributes["page_path"]
		_, err := c.GetPage(context.Background(), path)
		if err == nil {
			return fmt.Errorf("page %q still exists after destroy", path)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("unexpected error checking page %q: %w", path, err)
		}
	}
	return nil
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
