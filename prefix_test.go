package peek_test

import (
	"bytes"
	"testing"

	"github.com/git-pkgs/peek"
)

func assertClaimsPreserved(t testing.TB, prefix, extended peek.Result) {
	t.Helper()
	if extended.ClaimsLimited {
		// The output cap can omit otherwise valid claims.
		return
	}
	claims := make(map[peek.Claim]bool, len(extended.Claims))
	for _, claim := range extended.Claims {
		// Evidence can grow with the enclosing line.
		claim.Evidence = peek.Span{}
		claims[claim] = true
	}
	for _, claim := range prefix.Claims {
		claim.Evidence = peek.Span{}
		if !claims[claim] {
			t.Fatalf("prefix of %d bytes emitted a claim invalidated by extension: %+v", prefix.InputBytes, claim)
		}
	}
}

func TestControlByteAfterClaim(t *testing.T) {
	for _, prefix := range []string{
		"#!/usr/bin/python3 ", "# coding: utf-8 ", "https://example.org/ ",
		"/* SPDX-License-Identifier: MIT */", "<!-- SPDX-License-Identifier: MIT -->",
	} {
		for _, control := range []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 0x0b, 0x0e, 0x0f} {
			data := append([]byte(prefix), control)
			data = append(data, []byte("suffix\n")...)
			assertEveryCut(t, data)
			result := peek.Inspect(peek.Input{Bytes: data, Complete: true})
			if len(result.Claims) != 1 {
				t.Fatalf("%q: %+v", data, result)
			}
		}
	}
}

func FuzzPrefixSafety(f *testing.F) {
	for _, seed := range []string{
		"", "#!/usr/bin/env python3\n", "# coding: utf-8\n",
		"/* SPDX-License-Identifier: MIT-->*/\n",
		"<!-- SPDX-License-Identifier: MIT*/-->\n",
		"// Code generated tool; DO NOT EDIT.\n",
		"//go:build linux\n", "https://example.org/ \n",
		"<?xml encoding='UTF-8'?>\n", utf32LEBOM,
	} {
		f.Add([]byte(seed), uint32(len(seed)/2))
		f.Add([]byte(seed), uint32(len(seed)))
	}
	f.Add(bytes.Repeat([]byte("http://x \n"), peek.MaxClaims+1), uint32(peek.MaxClaims))
	f.Add(bytes.Repeat([]byte("x"), peek.MaxBytes+1), uint32(peek.MaxBytes))
	f.Fuzz(func(t *testing.T, data []byte, index uint32) {
		data = data[:min(len(data), peek.MaxBytes+1)]
		cut := int(index % uint32(len(data)+1))
		prefix := peek.Inspect(peek.Input{Bytes: data[:cut]})
		checkSpans(t, data[:cut], prefix)
		for _, complete := range []bool{false, true} {
			extended := peek.Inspect(peek.Input{Bytes: data, Complete: complete})
			checkSpans(t, data, extended)
			assertClaimsPreserved(t, prefix, extended)
		}
	})
}
