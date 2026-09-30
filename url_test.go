package peek_test

import (
	"testing"

	"github.com/git-pkgs/peek"
)

func TestURLLeadingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		prefix string
		count  int
	}{
		{"", 1}, {" ", 1}, {"\t", 1}, {"=", 1}, {"--flag=", 1},
		{"/", 1}, {"(", 1}, {"\"", 1},
		{"-", 0}, {"_", 0}, {".", 0}, {"see-", 0},
		{"a", 0}, {"Z", 0}, {"0", 0},
	} {
		t.Run(tc.prefix, func(t *testing.T) {
			for _, scheme := range []string{"http://", "https://"} {
				value := scheme + "example.org/path"
				data := []byte(tc.prefix + value + " \n")
				result := peek.Inspect(peek.Input{Bytes: data})
				if len(result.Claims) != tc.count {
					t.Fatalf("%q: got %+v, want %d claims", data, result.Claims, tc.count)
				}
				for _, claim := range result.Claims {
					if claim.Kind != peek.URL || string(data[claim.Value.Start:claim.Value.End]) != value {
						t.Fatalf("unexpected URL: %+v", claim)
					}
				}
				checkSpans(t, data, result)
				assertEveryCut(t, data)
			}
		})
	}
}
