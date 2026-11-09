// Package telegram supplies administrative notification source and manifest data.
// Registration, installed-state admission and owner adapters belong to the product.
package telegram

import (
	_ "embed"
	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
)

//go:embed component.json
var componentJSON []byte

// Manifest returns a fresh canonical descriptor without registering or starting work.
func Manifest() manifest.Manifest { return manifest.MustFromJSON(componentJSON) }
