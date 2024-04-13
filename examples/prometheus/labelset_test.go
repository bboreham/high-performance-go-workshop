package model

import (
	"testing"
)

func TestLabelSet_String(t *testing.T) {
	tests := []struct {
		input LabelSet
		want  string
	}{
		{
			input: nil,
			want:  `{}`,
		}, {
			input: LabelSet{
				"foo": "bar",
			},
			want: `{foo="bar"}`,
		}, {
			input: LabelSet{
				"foo":   "bar",
				"foo2":  "bar",
				"abc":   "prometheus",
				"foo11": "bar11",
			},
			want: `{abc="prometheus", foo="bar", foo11="bar11", foo2="bar"}`,
		},
	}
	for _, tt := range tests {
		t.Run("test", func(t *testing.T) {
			if got := tt.input.String(); got != tt.want {
				t.Errorf("LabelSet.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkLabelSetStringMethod(b *testing.B) {
	ls := LabelSet{
		"cluster": "primary",
		"foo":     "bar",
		"foo2":    "bar",
		"abc":     "prometheus",
		"foo11":   "bar11",
	}
	for i := 0; i < b.N; i++ {
		_ = ls.String()
	}
}
