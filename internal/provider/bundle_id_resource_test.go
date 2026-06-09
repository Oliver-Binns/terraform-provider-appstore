// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBundleIDResource(t *testing.T) {
	identifier := fmt.Sprintf("uk.co.oliverbinns.%s", uuid.New().String())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccBundleIDResourceConfig(identifier, "My Test App"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("identifier"),
						knownvalue.StringExact(identifier),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("My Test App"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("platform"),
						knownvalue.StringExact("UNIVERSAL"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("seed_id"),
						knownvalue.NotNull(),
					),
				},
			},
			// ImportState testing by identifier string
			{
				ResourceName:      "appstoreconnect_bundle_id.test",
				ImportState:       true,
				ImportStateId:     identifier,
				ImportStateVerify: true,
			},
			// ImportState with nonexistent identifier returns a clear error
			{
				ResourceName:  "appstoreconnect_bundle_id.test",
				ImportState:   true,
				ImportStateId: "uk.co.oliverbinns.does-not-exist",
				ExpectError:   regexp.MustCompile("Not Found"),
			},
			// Update name and read
			{
				Config: testAccBundleIDResourceConfig(identifier, "My Test App (renamed)"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("My Test App (renamed)"),
					),
					statecheck.ExpectKnownValue(
						"appstoreconnect_bundle_id.test",
						tfjsonpath.New("identifier"),
						knownvalue.StringExact(identifier),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccBundleIDResourceConfig(identifier, name string) string {
	return fmt.Sprintf(`
resource "appstoreconnect_bundle_id" "test" {
  identifier = %q
  name       = %q
  platform   = "UNIVERSAL"
}

variable "issuer_id" {
  type      = string
  sensitive = true
}

variable "key_id" {
  type      = string
  sensitive = true
}

variable "private_key" {
  type      = string
  sensitive = true
}

provider "appstoreconnect" {
  issuer_id   = var.issuer_id
  key_id      = var.key_id
  private_key = var.private_key
}
`, identifier, name)
}
