package peek_test

import (
	"fmt"

	"github.com/git-pkgs/peek"
)

func ExampleInspect() {
	data := []byte("// SPDX-License-Identifier: MIT")
	result := peek.Inspect(peek.Input{Bytes: data, Complete: true})
	for _, claim := range result.Claims {
		fmt.Printf("%s: %s\n", claim.Kind, data[claim.Value.Start:claim.Value.End])
	}
	// Output:
	// spdx-license: MIT
}

func ExampleInspectInto() {
	data := []byte("// SPDX-License-Identifier: MIT\n// Copyright 2026 Example\npackage exam")
	var result peek.Result
	peek.InspectInto(&result, peek.Input{Bytes: data})
	for _, claim := range result.Claims {
		fmt.Printf("%s: %s\n", claim.Kind, data[claim.Value.Start:claim.Value.End])
	}
	// Output:
	// spdx-license: MIT
	// copyright: Copyright 2026 Example
}
