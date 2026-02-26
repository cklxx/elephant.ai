package agent_eval

import (
	"reflect"
	"testing"
)

func TestParseCSVTags(t *testing.T) {
	t.Run("empty input returns nil", func(t *testing.T) {
		got := ParseCSVTags("")
		if got != nil {
			t.Fatalf("ParseCSVTags(\"\") = %#v, want nil", got)
		}
	})

	t.Run("trim and drop empty tags", func(t *testing.T) {
		got := ParseCSVTags("alpha, beta , ,gamma")
		want := []string{"alpha", "beta", "gamma"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ParseCSVTags returned %#v, want %#v", got, want)
		}
	})

	t.Run("delimiters only returns empty non-nil slice", func(t *testing.T) {
		got := ParseCSVTags(" , , ")
		if got == nil {
			t.Fatal("ParseCSVTags returned nil, want empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("ParseCSVTags length = %d, want 0", len(got))
		}
	})
}
