package parsers

import "github.com/sharedco/cilo/pkg/engine"

func init() {
	// Register all parsers with the default registry
	engine.Register(&ComposeParser{})
}
