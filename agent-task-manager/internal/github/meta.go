package github

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// metaBlock stores the atama-only fields (depends_on, goal_ids) that have no
// natural GitHub Issue field, embedded as an HTML comment appended to the
// Issue body (requirements §5.4: "本文中に構造化コメントとして埋め込む").
// HTML comments render invisibly on GitHub, so the block doesn't clutter
// the Issue for human readers.
type metaBlock struct {
	AtamaID   string   `yaml:"atama_id"`
	DependsOn []string `yaml:"depends_on,omitempty"`
	GoalIDs   []string `yaml:"goal_ids,omitempty"`
}

var metaBlockPattern = regexp.MustCompile(`(?s)\n?<!-- atama:meta\n(.*?)\n-->\s*\z`)

// appendMeta returns body with meta's fields appended as a trailing HTML
// comment block, replacing any block already present.
func appendMeta(body string, meta metaBlock) string {
	clean, _ := stripMeta(body)
	data, err := yaml.Marshal(meta)
	if err != nil {
		// meta is a plain struct of strings; Marshal cannot fail here.
		panic(err)
	}
	block := "\n<!-- atama:meta\n" + strings.TrimRight(string(data), "\n") + "\n-->\n"
	return strings.TrimRight(clean, "\n") + block
}

// stripMeta extracts a trailing atama:meta block from body, if present, and
// returns the body with that block removed.
func stripMeta(body string) (clean string, meta *metaBlock) {
	loc := metaBlockPattern.FindStringSubmatch(body)
	if loc == nil {
		return body, nil
	}
	var m metaBlock
	if err := yaml.Unmarshal([]byte(loc[1]), &m); err != nil {
		return body, nil
	}
	clean = metaBlockPattern.ReplaceAllString(body, "")
	return clean, &m
}
