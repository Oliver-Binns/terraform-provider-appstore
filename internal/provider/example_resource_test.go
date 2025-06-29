// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccExampleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccExampleResourceConfig("one"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"appstoreconnect_example.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("example-id"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_example.test",
						tfjsonpath.New("defaulted"),
						knownvalue.StringExact("example value when not configured"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_example.test",
						tfjsonpath.New("configurable_attribute"),
						knownvalue.StringExact("one"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "appstoreconnect_example.test",
				ImportState:       true,
				ImportStateVerify: true,
				// This is not normally necessary, but is here because this
				// example code does not have an actual upstream service.
				// Once the Read method is able to refresh information from
				// the upstream service, this can be removed.
				ImportStateVerifyIgnore: []string{"configurable_attribute", "defaulted"},
			},
			// Update and Read testing
			{
				Config: testAccExampleResourceConfig("two"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"appstoreconnect_example.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("example-id"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_example.test",
						tfjsonpath.New("defaulted"),
						knownvalue.StringExact("example value when not configured"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_example.test",
						tfjsonpath.New("configurable_attribute"),
						knownvalue.StringExact("two"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccExampleResourceConfig(configurableAttribute string) string {
	return fmt.Sprintf(`
resource "appstoreconnect_example" "test" {
  configurable_attribute = %[1]q
}

provider "appstoreconnect" {
  issuer_id   = "4389f85c-98c6-4023-ab25-8154fcd9460d"
  key_id      = "A1234B5678"
  private_key = <<EOF
-----BEGIN PRIVATE KEY-----
MHcCAQEEIG706QZ+qBP9FxNbs8lVhIf0w/hJJ+pMu6YtG/d8uqnkoAoGCCqGSM49
AwEHoUQDQgAEnMKTGhM0U4Q5rCvgobWZQtcmknAEZOxTqjmtJf1jlTfHO7iLykAj
AoyVWzvsnOZ2F3ujWssdv6b27lkdrm513w==
-----END PRIVATE KEY-----
EOF
}
`, configurableAttribute)
}
