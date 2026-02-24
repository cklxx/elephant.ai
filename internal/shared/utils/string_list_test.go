package utils

import (
	"reflect"
	"testing"
)

func TestTrimDedupeStrings(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "nil input",
			input:  nil,
			expect: nil,
		},
		{
			name:   "empty input",
			input:  []string{},
			expect: []string{},
		},
		{
			name:   "trim and dedupe",
			input:  []string{" A ", "B", "A", "\tB", "  ", "C"},
			expect: []string{"A", "B", "C"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := TrimDedupeStrings(test.input)
			if !reflect.DeepEqual(got, test.expect) {
				t.Fatalf("TrimDedupeStrings(%v) = %#v; want %#v", test.input, got, test.expect)
			}
		})
	}
}
