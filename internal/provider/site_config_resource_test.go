package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSiteConfigResource(t *testing.T) {
	const key = "siteName"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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

func testAccSiteConfigString(key, value string) string {
	return fmt.Sprintf(`
resource "kener_site_config" "test" {
  key   = %[1]q
  value = jsonencode(%[2]q)
}
`, key, value)
}
