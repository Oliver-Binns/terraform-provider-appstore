// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccExampleResource(t *testing.T) {
	accountEmail := fmt.Sprintf(
		"%s@oliverbinns.co.uk",
		uuid.New().String(),
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccExampleResourceConfig(accountEmail),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"appstoreconnect_user.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("example-id"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_user.test",
						tfjsonpath.New("email"),
						knownvalue.StringExact(accountEmail),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_user.test",
						tfjsonpath.New("first_name"),
						knownvalue.StringExact("John"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_user.test",
						tfjsonpath.New("last_name"),
						knownvalue.StringExact("Smith"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_user.test",
						tfjsonpath.New("all_apps_visible"),
						knownvalue.Bool(false),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_user.test",
						tfjsonpath.New("provisioning_allowed"),
						knownvalue.Bool(false),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:      "appstoreconnect_example.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and read: not currently supported, but this is how it would look
			/*{
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
			},*/
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccExampleResourceConfig(accountEmail string) string {
	return fmt.Sprintf(`
resource "appstoreconnect_user" "test" {
  first_name = "John"
  last_name  = "Smith"

  email = "%s"

  all_apps_visible     = false
  provisioning_allowed = false
}

variable "issuer_id" {
  type        = string
  sensitive   = true
}

variable "key_id" {
  type        = string
  sensitive   = true
}

variable "private_key" {
  type        = string
  sensitive   = true
}

provider "appstoreconnect" {
  issuer_id   = var.issuer_id
  key_id      = var.key_id
  private_key = var.private_key
}
`, accountEmail)
}
