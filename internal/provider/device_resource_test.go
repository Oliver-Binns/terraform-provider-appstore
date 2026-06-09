// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	iphone16ProUDID = "00008140-000A159C2013C01C"
	iphone16ProName = "Oliver's iPhone 16 Pro"
)

func TestAccDeviceResource_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Import by UDID and verify all fields are correctly populated.
			// ImportStatePersist defaults to false so state is cleared after
			// this step — the framework has nothing to destroy.
			{
				ResourceName:  "appstoreconnect_device.test",
				ImportState:   true,
				ImportStateId: iphone16ProUDID,
				Config:        testAccDeviceResourceConfig(iphone16ProName),
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported state, got %d", len(states))
					}
					attrs := states[0].Attributes
					checks := map[string]string{
						"udid":     iphone16ProUDID,
						"platform": "IOS",
						"name":     iphone16ProName,
					}
					for attr, expected := range checks {
						if got := attrs[attr]; got != expected {
							return fmt.Errorf("expected %s=%q, got %q", attr, expected, got)
						}
					}
					for _, attr := range []string{"id", "device_class", "model", "status"} {
						if attrs[attr] == "" {
							return fmt.Errorf("expected %s to be set", attr)
						}
					}
					return nil
				},
			},
		},
	})
}

func testAccDeviceResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "appstoreconnect_device" "test" {
  name     = %q
  udid     = "%s"
  platform = "IOS"
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
`, name, iphone16ProUDID)
}
