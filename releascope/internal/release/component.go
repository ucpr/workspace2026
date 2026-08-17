package release

import (
	"regexp"
	"sort"
	"strings"
)

// componentKinds are the OpenTelemetry Collector Contrib component
// categories called out in spec section 9.
var componentKinds = []string{"receiver", "processor", "exporter", "connector", "extension"}

// bracketPattern matches opentelemetry-collector-contrib's PR title
// convention, e.g. "[receiver/prometheus]", "[exporter/datadog]", or
// "[processor/k8sattributes, exporter/otlphttp]" for multi-component PRs.
var bracketPattern = regexp.MustCompile(`(?i)\[([a-z0-9_/,.\s-]+)\]`)

// bracketEntryPattern splits a single "kind/name" pair out of a bracket
// group's comma-separated contents.
var bracketEntryPattern = regexp.MustCompile(`(?i)^\s*(receiver|processor|exporter|connector|extension)\s*/\s*([a-z0-9_]+)\s*$`)

// directPattern matches a component name written out directly, e.g.
// "prometheusreceiver" or "k8sattributesprocessor", appearing anywhere
// in free text such as a release note body.
var directPattern = regexp.MustCompile(`(?i)\b([a-z0-9]+(?:receiver|processor|exporter|connector|extension))\b`)

// ExtractComponents scans PR titles/bodies and release note text for
// OpenTelemetry Collector Contrib component names. This is a best-effort
// heuristic, not a precise parser (spec section 9 explicitly does not
// require full accuracy for the MVP): it is meant to populate
// `affectedComponents` well enough to be useful for the future
// "user collector config impact analysis" feature (spec section 32).
func ExtractComponents(texts ...string) []string {
	found := map[string]struct{}{}

	for _, text := range texts {
		for _, group := range bracketPattern.FindAllStringSubmatch(text, -1) {
			for _, entry := range strings.Split(group[1], ",") {
				if m := bracketEntryPattern.FindStringSubmatch(entry); m != nil {
					found[strings.ToLower(m[2]+m[1])] = struct{}{}
				}
			}
		}
		for _, m := range directPattern.FindAllStringSubmatch(text, -1) {
			name := strings.ToLower(m[1])
			if isKnownKindSuffix(name) {
				found[name] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func isKnownKindSuffix(name string) bool {
	for _, kind := range componentKinds {
		if strings.HasSuffix(name, kind) && len(name) > len(kind) {
			return true
		}
	}
	return false
}
