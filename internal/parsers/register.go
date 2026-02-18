// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package parsers

import "github.com/sharedco/cilo/internal/engine"

func init() {
	// Register all parsers with the default registry
	engine.Register(&ComposeParser{})
}
