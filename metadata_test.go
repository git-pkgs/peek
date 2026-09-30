package peek_test

import (
	"testing"

	"github.com/git-pkgs/peek"
)

func TestMetadataCommentTerminators(t *testing.T) {
	const license = "MIT"
	const cValue = license + "*/"
	const htmlValue = license + "-->"
	for _, tc := range []struct {
		name, data, value string
		closed            bool
	}{
		{"bare C closer", "SPDX-License-Identifier: MIT*/", cValue, false},
		{"bare HTML closer", "SPDX-License-Identifier: MIT-->", htmlValue, false},
		{"slash comment", "// SPDX-License-Identifier: MIT*/", cValue, false},
		{"hash comment", "# SPDX-License-Identifier: MIT-->", htmlValue, false},
		{"semicolon comment", "; SPDX-License-Identifier: MIT*/", cValue, false},
		{"dash comment", "-- SPDX-License-Identifier: MIT-->", htmlValue, false},
		{"C comment", "/* SPDX-License-Identifier: MIT*/", license, true},
		{"HTML comment", "<!-- SPDX-License-Identifier: MIT-->", license, true},
		{"C continuation", "/*\n * SPDX-License-Identifier: MIT*/", license, true},
		{"C mismatched closer", "/* SPDX-License-Identifier: MIT-->", htmlValue, false},
		{"HTML mismatched closer", "<!-- SPDX-License-Identifier: MIT*/", cValue, false},
		{"continuation mismatched closer", "/*\n * SPDX-License-Identifier: MIT-->", htmlValue, false},
		{"C mixed closers", "/* SPDX-License-Identifier: MIT-->*/", htmlValue, true},
		{"HTML mixed closers", "<!-- SPDX-License-Identifier: MIT*/-->", cValue, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, ending := range []struct {
				text     string
				complete bool
			}{
				{"", false}, {"", true}, {"\n", false}, {"\r\n", false},
			} {
				data := []byte(tc.data + ending.text)
				result := peek.Inspect(peek.Input{Bytes: data, Complete: ending.complete})
				want := 1
				if !tc.closed && ending.text == "" && !ending.complete {
					want = 0
				}
				if len(result.Claims) != want {
					t.Fatalf("ending=%q complete=%v: %+v", ending.text, ending.complete, result)
				}
				for _, claim := range result.Claims {
					if claim.Kind != peek.SPDXLicense || string(data[claim.Value.Start:claim.Value.End]) != tc.value {
						t.Fatalf("got %+v, want license %q", claim, tc.value)
					}
				}
				checkSpans(t, data, result)
			}
			assertEveryCut(t, []byte(tc.data+" extra\n"))
		})
	}
}
