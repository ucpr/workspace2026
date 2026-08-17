package release

import (
	"reflect"
	"testing"
)

func TestExtractComponents(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "bracketed PR title",
			in:   []string{"[receiver/prometheus] Fix label collision"},
			want: []string{"prometheusreceiver"},
		},
		{
			name: "multiple components in one bracket",
			in:   []string{"[processor/k8sattributes, exporter/otlphttp] Support new field"},
			want: []string{"k8sattributesprocessor", "otlphttpexporter"},
		},
		{
			name: "direct mention in release note body",
			in:   []string{"The datadogexporter now supports metric remapping."},
			want: []string{"datadogexporter"},
		},
		{
			name: "dedupes across multiple texts",
			in: []string{
				"[receiver/prometheus] initial support",
				"prometheusreceiver: fix flaky test",
			},
			want: []string{"prometheusreceiver"},
		},
		{
			name: "no components mentioned",
			in:   []string{"Bump golang.org/x/net from 0.1 to 0.2"},
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractComponents(tc.in...)
			if len(got) == 0 {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtractComponents(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
