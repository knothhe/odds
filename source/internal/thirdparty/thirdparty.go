// Package thirdparty embeds the license texts collected from linked packages.
package thirdparty

import (
	_ "embed"
	"encoding/json"
)

//go:embed licenses.json
var catalogJSON []byte

type Text struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type Dependency struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Texts   []Text `json:"texts"`
}

// Parse once at startup; UI frames never read files or scan dependencies.
var Dependencies = func() []Dependency {
	var entries []Dependency
	if err := json.Unmarshal(catalogJSON, &entries); err != nil {
		panic(err) // A malformed embedded build resource is a build defect.
	}
	return entries
}()
