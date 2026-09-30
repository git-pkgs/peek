package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/peek"
)

const bytesFlag = "-bytes"
const prefixFlag = "-prefix"

func TestCLIJSONContract(t *testing.T) {
	for _, tc := range []struct{ input, output string }{
		{"", `{"path":"-","input_bytes":0,"input_complete":true,"scanned_bytes":0,"bytes_limited":false,"claims_limited":false,"claims":[]}`},
		{"\xef\xbb\xbf", `{"path":"-","input_bytes":3,"input_complete":true,"scanned_bytes":3,"bytes_limited":false,"claims_limited":false,"claims":[{"kind":"bom","rule":"byte-order-mark","evidence":{"start":0,"end":3},"value":{"start":0,"end":3},"label":"utf-8","text":"\ufeff"}]}`},
	} {
		var output bytes.Buffer
		if err := run(nil, strings.NewReader(tc.input), &output, io.Discard); err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err := json.Unmarshal(output.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(tc.output), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s, want %s", output.Bytes(), tc.output)
		}
	}
}

func TestCLICompleteness(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		args       []string
		complete   bool
		count      int
	}{
		{"EOF", "# SPDX-License-Identifier: MIT", nil, true, 1},
		{"prefix", "# SPDX-License-Identifier: MIT", []string{prefixFlag}, false, 0},
		{"boundary", "# SPDX-License-Identifier: MIT OR BSD-3-Clause", []string{bytesFlag, "29"}, false, 0},
		{"exact", "#!/bin/sh", []string{bytesFlag, "9"}, true, 2},
		{"longer", "#!/bin/shx", []string{bytesFlag, "9"}, false, 0},
		{"newline", "#!/bin/sh\n", []string{prefixFlag}, false, 2},
		{"directive EOF", "//go:build linux && arm64", nil, true, 1},
		{"directive prefix", "//go:build linux && arm64", []string{prefixFlag}, false, 0},
		{"directive newline", "//go:build linux && arm64\n", []string{prefixFlag}, false, 1},
		{"unmatched closer prefix", "SPDX-License-Identifier: MIT*/", []string{prefixFlag}, false, 0},
		{"matched closer prefix", "/* SPDX-License-Identifier: MIT*/", []string{prefixFlag}, false, 1},
		{"continuation closer prefix", "/*\n * SPDX-License-Identifier: MIT*/", []string{prefixFlag}, false, 1},
		{"control after interpreter", "#!/bin/sh \x00suffix", nil, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output, stderr bytes.Buffer
			if err := run(tc.args, strings.NewReader(tc.data), &output, &stderr); err != nil {
				t.Fatal(err)
			}
			var result outputJSON
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.InputComplete != tc.complete || len(result.Claims) != tc.count {
				t.Fatalf("%s", output.Bytes())
			}
		})
	}
}

func TestCLIFilesAndErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{path, path}, nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	for range 2 {
		var result outputJSON
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.Path != path || len(result.Claims) != 2 {
			t.Fatalf("%+v", result)
		}
	}
	for _, args := range [][]string{{bytesFlag, "0"}, {bytesFlag, "65537"}, {"-unknown"}, {path + "missing"}} {
		if err := run(args, strings.NewReader(""), io.Discard, io.Discard); err == nil {
			t.Fatalf("expected error: %v", args)
		}
	}
	if err := run([]string{"-h"}, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

type failedIO struct{}

func (failedIO) Read([]byte) (int, error)  { return 0, errors.New("read failure") }
func (failedIO) Write([]byte) (int, error) { return 0, errors.New("write failure") }

func TestCLIIOErrors(t *testing.T) {
	if err := run(nil, failedIO{}, io.Discard, io.Discard); err == nil {
		t.Fatal("missing read error")
	}
	if err := run(nil, strings.NewReader(""), failedIO{}, io.Discard); err == nil {
		t.Fatal("missing write error")
	}
}

func TestCLIReadBound(t *testing.T) {
	reader := strings.NewReader("#!/bin/sh\necho hello\n")
	before := reader.Len()
	if err := run([]string{bytesFlag, "9"}, reader, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := before - reader.Len(); got != 10 {
		t.Fatalf("read %d bytes, want limit plus one", got)
	}
}

func TestCLIInvalidUTF8(t *testing.T) {
	data := []byte("# Copyright 2026 \xff\n")
	var output bytes.Buffer
	if err := run(nil, bytes.NewReader(data), &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	var result outputJSON
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Claims) != 1 || result.Claims[0].Kind != peek.Copyright || !bytes.Equal(result.Claims[0].Raw, data[2:len(data)-1]) {
		t.Fatalf("%+v", result)
	}
}
