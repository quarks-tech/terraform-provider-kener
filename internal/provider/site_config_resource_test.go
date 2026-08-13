package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSiteConfigResource(t *testing.T) {
	const key = "siteName"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSiteConfigDestroy,
		Steps: []resource.TestStep{
			// Set a string-valued key.
			{
				Config: testAccSiteConfigString(key, "TF Acc Site"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_site_config.test", "key", key),
					resource.TestCheckResourceAttr("kener_site_config.test", "value", `"TF Acc Site"`),
					resource.TestCheckResourceAttr("kener_site_config.test", "data_type", "string"),
				),
			},
			// Import by key.
			{
				ResourceName:                         "kener_site_config.test",
				ImportState:                          true,
				ImportStateId:                        key,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "key",
			},
			// Update the value in place.
			{
				Config: testAccSiteConfigString(key, "TF Acc Site Renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kener_site_config.test", "value", `"TF Acc Site Renamed"`),
				),
			},
		},
	})
}

// testAccCheckSiteConfigDestroy asserts the intentional no-op delete: site-config
// keys cannot be removed, so after destroy the key must still resolve on the
// server (the resource is only dropped from Terraform state).
func testAccCheckSiteConfigDestroy(s *terraform.State) error {
	c, err := testAccClient()
	if err != nil {
		return err
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kener_site_config" {
			continue
		}
		key := rs.Primary.Attributes["key"]
		if _, err := c.GetSiteConfig(context.Background(), key); err != nil {
			return fmt.Errorf("site config %q should remain after destroy, but reading it failed: %w", key, err)
		}
	}
	return nil
}

func testAccSiteConfigString(key, value string) string {
	return fmt.Sprintf(`
resource "kener_site_config" "test" {
  key   = %[1]q
  value = jsonencode(%[2]q)
}
`, key, value)
}
