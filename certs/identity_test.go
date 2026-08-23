package certs

import "testing"

func TestDisplayNamesNormalizeSpacesAndPreserveCase(t *testing.T) {
	definition, err := NewIssuerDefinition("  Foo   Agent  ")
	if err != nil {
		t.Fatal(err)
	}
	if definition.Name != "Foo Agent" || definition.Slug != "foo-agent" {
		t.Fatalf("unexpected definition: %+v", definition)
	}
	if _, err = NewIssuerDefinition("Foo\tAgent"); err == nil {
		t.Fatal("tab was accepted in a display name")
	}
}
