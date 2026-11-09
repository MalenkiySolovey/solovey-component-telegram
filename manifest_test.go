package telegram

import "testing"

func TestManifestPreservesPublicIdentityAndOwnerBoundaries(t *testing.T) {
	descriptor := Manifest()
	if descriptor.ID != "telegram" || descriptor.Version != "1" || descriptor.Delivery != "in-process" || !descriptor.DefaultEnabled {
		t.Fatalf("component identity changed: %+v", descriptor)
	}
	if descriptor.HostAPI == nil || descriptor.HostAPI.Version != "solovey.component-host/v1" {
		t.Fatal("Host API changed")
	}
	if len(descriptor.Database.Settings) != 12 || len(descriptor.Database.Secrets) != 5 {
		t.Fatal("durable setting/secret ownership changed")
	}
	descriptor.ID = "changed"
	if Manifest().ID != "telegram" {
		t.Fatal("caller mutated canonical manifest")
	}
}
