package peek_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/git-pkgs/peek"
)

func TestCorpus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
	}{
		{"python", []string{"/usr/bin/env", "/usr/bin/env -S python3 -u", "-S python3 -u", "utf-8", "Apache-2.0 OR MIT", "2026 Example Contributors", "https://example.org/tooling"}},
		{"generated", []string{"by protoc-gen-go.", "schema/message.proto", "BSD-3-Clause"}},
		{"c", []string{"(MIT OR Apache-2.0)", "Copyright (c) 2026 Example Contributors", "#pragma once", "schema/message.idl", "https://example.org/protocol"}},
		{"xml", []string{"<?xml version=\"1.0\"\n      encoding='UTF-8'?>", "UTF-8", "MIT", "https://example.org/schema"}},
		{"build", []string{"//go:build linux && arm64", "//go:generate stringer -type=State", "Copyright 2026 Example Authors"}},
		{"reuse", []string{"LicenseRef-Example", "MIT"}},
		{"plain", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.name+".prefix"))
			if err != nil {
				t.Fatal(err)
			}
			result := peek.Inspect(peek.Input{Bytes: data})
			var values []string
			for _, claim := range result.Claims {
				values = append(values, string(data[claim.Value.Start:claim.Value.End]))
			}
			if !reflect.DeepEqual(values, tc.values) {
				t.Fatalf("got %q, want %q", values, tc.values)
			}
			checkSpans(t, data, result)
			assertEveryCut(t, data)
		})
	}
}

func assertEveryCut(t *testing.T, data []byte) {
	t.Helper()
	full := peek.Inspect(peek.Input{Bytes: data, Complete: true})
	for cut := range len(data) + 1 {
		prefix := peek.Inspect(peek.Input{Bytes: data[:cut]})
		checkSpans(t, data[:cut], prefix)
		for _, claim := range prefix.Claims {
			found := false
			for _, other := range full.Claims {
				if claim.Kind == other.Kind && claim.Rule == other.Rule && claim.Value == other.Value && claim.Label == other.Label {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("cut %d emitted a claim that later bytes invalidate: %+v", cut, claim)
			}
		}
	}
}

func checkSpans(t testing.TB, data []byte, result peek.Result) {
	t.Helper()
	if result.InputBytes != len(data) || result.ScannedBytes > peek.MaxBytes || len(result.Claims) > peek.MaxClaims {
		t.Fatalf("invalid limits: %+v", result)
	}
	for _, claim := range result.Claims {
		if claim.Evidence.Start < 0 || claim.Evidence.Start > claim.Value.Start || claim.Value.Start >= claim.Value.End || claim.Value.End > claim.Evidence.End || claim.Evidence.End > result.ScannedBytes {
			t.Fatalf("invalid claim: %+v", claim)
		}
		if claim.Rule == "" || claim.Kind == "" {
			t.Fatalf("missing provenance: %+v", claim)
		}
	}
}

func TestCutCannotShortenLicense(t *testing.T) {
	data := []byte("// SPDX-License-Identifier: MIT OR Apache-2.0\n")
	for cut := len("// SPDX-License-Identifier: MIT"); cut < len(data); cut++ {
		result := peek.Inspect(peek.Input{Bytes: data[:cut]})
		if len(result.Claims) != 0 {
			t.Fatalf("cut %d: %+v", cut, result.Claims)
		}
	}
	complete := peek.Inspect(peek.Input{Bytes: []byte("// SPDX-License-Identifier: MIT"), Complete: true})
	if len(complete.Claims) != 1 {
		t.Fatalf("%+v", complete)
	}
}

func TestUnseenMetadataIsNotAbsent(t *testing.T) {
	const window = 1024
	data := append(bytes.Repeat([]byte("\n"), window), []byte("# SPDX-License-Identifier: MIT\n")...)
	prefix := peek.Inspect(peek.Input{Bytes: data[:window]})
	full := peek.Inspect(peek.Input{Bytes: data, Complete: true})
	if prefix.InputComplete || len(prefix.Claims) != 0 || !full.InputComplete || len(full.Claims) != 1 {
		t.Fatalf("prefix=%+v full=%+v", prefix, full)
	}
}

func TestMetadataBoundaries(t *testing.T) {
	for _, tc := range []struct {
		data  string
		count int
	}{
		{"/* SPDX-License-Identifier: MIT */truncated", 1},
		{"<!-- SPDX-License-Identifier: MIT -->", 1},
		{"# SPDX-License-Identifier: ", 0},
		{"#!/bin/py", 0},
		{"#!/bin/python ", 1},
		{"# coding: utf-", 0},
		{"# coding: utf-8 ", 1},
		{"# coding: \n", 0},
		{"\n\n# coding: utf-8\n", 0},
		{"const x = \"# coding: utf-8\"\n", 0},
		{"https://example.org/trunc", 0},
		{"https://example.org/closed \"trunc", 1},
		{"https://example.org/closed*/", 1},
		{"http:// \n", 0},
		{"nothttps://example.org \n", 0},
		{"#!/bin/sh\r\n# SPDX-License-Identifier: MIT\r\n", 3},
		{"\x00# SPDX-License-Identifier: MIT\n", 0},
		{"<?xml version=\"1.0\" encoding=\"UTF-8\"", 0},
		{"<?xml-stylesheet href=\"file\"?>", 0},
		{"<?xml version=\"1.0\" encoding=\"UTF-8\"?>", 2},
		{"<?xml version=\"encoding='fake'\"?>", 1},
	} {
		t.Run(tc.data, func(t *testing.T) {
			data := []byte(tc.data)
			result := peek.Inspect(peek.Input{Bytes: data})
			if len(result.Claims) != tc.count {
				t.Fatalf("got %+v, want %d claims", result.Claims, tc.count)
			}
			checkSpans(t, data, result)
		})
	}
}

func TestHeaderConventions(t *testing.T) {
	for _, prefix := range []string{"", "//", "#", ";", "--", "/*", "*", "<!--"} {
		data := []byte(prefix + " SPDX-License-Identifier: MIT\n")
		result := peek.Inspect(peek.Input{Bytes: data})
		if len(result.Claims) != 1 || result.Claims[0].Kind != peek.SPDXLicense {
			t.Fatalf("%q: %+v", prefix, result)
		}
	}
	for _, notice := range []string{"copyright 2026 Example", "COPYRIGHT 2026 Example", "Copyright 2026 Example"} {
		result := peek.Inspect(peek.Input{Bytes: []byte("// " + notice + "\n")})
		if len(result.Claims) != 1 || result.Claims[0].Kind != peek.Copyright {
			t.Fatalf("%q: %+v", notice, result)
		}
	}
}

func TestBOMs(t *testing.T) {
	for _, tc := range []struct{ data, label string }{
		{"\xef\xbb\xbf", "utf-8"}, {"\xff\xfeA\x00", "utf-16le"},
		{"\xfe\xff\x00A", "utf-16be"}, {"\xff\xfe\x00\x00", "utf-32le"},
		{"\x00\x00\xfe\xff", "utf-32be"},
	} {
		data := []byte(tc.data)
		result := peek.Inspect(peek.Input{Bytes: data})
		if len(result.Claims) != 1 || result.Claims[0].Label != tc.label {
			t.Fatalf("%+v", result)
		}
		assertEveryCut(t, data)
	}
	data := []byte("\xef\xbb\xbf# SPDX-License-Identifier: MIT\n")
	result := peek.Inspect(peek.Input{Bytes: data})
	if len(result.Claims) != 2 || result.Claims[1].Evidence.Start != 3 {
		t.Fatalf("%+v", result)
	}
	assertEveryCut(t, data)
}

func TestLimitsAndReuse(t *testing.T) {
	data := bytes.Repeat([]byte("https://example.org/ \n"), peek.MaxClaims+1)
	result := peek.Inspect(peek.Input{Bytes: data, Complete: true})
	if len(result.Claims) != peek.MaxClaims || !result.ClaimsLimited {
		t.Fatalf("claim limit: %+v", result)
	}
	checkSpans(t, data, result)
	peek.InspectInto(&result, peek.Input{})
	if len(result.Claims) != 0 || result.ClaimsLimited || result.InputComplete {
		t.Fatalf("stale result: %+v", result)
	}
	data = bytes.Repeat([]byte(" "), peek.MaxBytes+100)
	copy(data[peek.MaxBytes-len("# SPDX-License-Identifier: MIT"):], "# SPDX-License-Identifier: MIT OR Apache-2.0\n")
	peek.InspectInto(&result, peek.Input{Bytes: data, Complete: true})
	if !result.BytesLimited || !result.InputComplete || len(result.Claims) != 0 {
		t.Fatalf("byte limit: %+v", result)
	}
}

func TestReusableClaimsDoNotAllocate(t *testing.T) {
	input := peek.Input{Bytes: benchmarkInput("#!/bin/sh\n# SPDX-License-Identifier: MIT\n")}
	result := peek.Inspect(input)
	if allocations := testing.AllocsPerRun(100, func() { peek.InspectInto(&result, input) }); allocations != 0 {
		t.Fatalf("%g allocations", allocations)
	}
}

func TestConcurrentInspect(t *testing.T) {
	input := peek.Input{Bytes: benchmarkInput("#!/bin/sh\n# SPDX-License-Identifier: MIT\n")}
	want := peek.Inspect(input)
	for range 8 {
		t.Run("worker", func(t *testing.T) {
			t.Parallel()
			var result peek.Result
			for range 100 {
				peek.InspectInto(&result, input)
				if !reflect.DeepEqual(result, want) {
					t.Fatal("concurrent result changed")
				}
			}
		})
	}
}

func FuzzInspect(f *testing.F) {
	for _, seed := range []string{"", "#!/bin/sh\n", "# SPDX-License-Identifier: MIT OR BSD-3-Clause", "<?xml encoding='UTF-8'?>", "https://example.org/ ", "\xff\xfe\x00\x00"} {
		f.Add([]byte(seed), false)
	}
	f.Fuzz(func(t *testing.T, data []byte, complete bool) {
		result := peek.Inspect(peek.Input{Bytes: data, Complete: complete})
		checkSpans(t, data, result)
		again := peek.Inspect(peek.Input{Bytes: data, Complete: complete})
		if !reflect.DeepEqual(result, again) {
			t.Fatal("nondeterministic result")
		}
	})
}
