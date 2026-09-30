package peek_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/git-pkgs/peek"
)

func TestInspectStorage(t *testing.T) {
	data := []byte("// SPDX-License-Identifier: MIT\n")
	original := bytes.Clone(data)
	first := peek.Inspect(peek.Input{Bytes: data})
	second := peek.Inspect(peek.Input{Bytes: data})
	if !bytes.Equal(data, original) || !reflect.DeepEqual(first, second) || len(first.Claims) != 1 {
		t.Fatal("inspection changed the input or its results")
	}
	first.Claims[0].Rule = "changed"
	if second.Claims[0].Rule != "spdx-license-tag" {
		t.Fatal("Inspect results share claim storage")
	}
	retained := append([]peek.Claim(nil), second.Claims...)
	peek.InspectInto(&second, peek.Input{Bytes: []byte("#!/bin/sh\n")})
	if retained[0].Kind != peek.SPDXLicense || retained[0].Rule != "spdx-license-tag" {
		t.Fatal("copied claims changed on reuse")
	}
}

func TestShebangBoundary(t *testing.T) {
	data := []byte("#!/usr/bin/env -S python3 -u")
	for _, complete := range []bool{false, true} {
		result := peek.Inspect(peek.Input{Bytes: data, Complete: complete})
		want := 1
		if complete {
			want = 3
		}
		if len(result.Claims) != want {
			t.Fatalf("complete=%v: %+v", complete, result)
		}
		if got := string(data[result.Claims[0].Value.Start:result.Claims[0].Value.End]); got != "/usr/bin/env" {
			t.Fatal(got)
		}
	}
}

func TestDirectiveBoundaries(t *testing.T) {
	for _, directive := range []string{"//go:build linux && arm64", "//go:generate stringer -type=State", "#pragma once"} {
		t.Run(directive, func(t *testing.T) {
			for _, tc := range []struct {
				ending   string
				complete bool
				count    int
			}{
				{"", false, 0}, {"", true, 1},
				{"\n", false, 1}, {"\n", true, 1},
				{"\r\n", false, 1}, {"\r\n", true, 1},
			} {
				data := []byte(" \t" + directive + "\t" + tc.ending)
				result := peek.Inspect(peek.Input{Bytes: data, Complete: tc.complete})
				if len(result.Claims) != tc.count {
					t.Fatalf("ending=%q complete=%v: %+v", tc.ending, tc.complete, result)
				}
				for _, claim := range result.Claims {
					if claim.Kind != peek.Directive || string(data[claim.Value.Start:claim.Value.End]) != directive {
						t.Fatalf("unexpected directive: %+v", claim)
					}
				}
				checkSpans(t, data, result)
			}
			assertEveryCut(t, []byte(directive+"\n"))
		})
	}
}

func TestDirectiveNonMatches(t *testing.T) {
	for _, line := range []string{
		"", " \t", "/", "//", "#", "// ordinary source line", "# ordinary comment",
		"//go:", "//go:build", "//go:buildx linux", "//go:generate", "//go:generatex tool",
		"#pragma", "#pragmatic once", "// go:build linux", "/*go:build linux */", "pragma once",
	} {
		t.Run(line, func(t *testing.T) {
			data := []byte(" \t" + line + "\n")
			result := peek.Inspect(peek.Input{Bytes: data})
			if len(result.Claims) != 0 {
				t.Fatalf("unexpected claims: %+v", result.Claims)
			}
			assertEveryCut(t, data)
		})
	}
}
