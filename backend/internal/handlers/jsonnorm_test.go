package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

type normInner struct {
	Tags []string `json:"tags"`
}

type normOuter struct {
	Items    []int             `json:"items"`
	Optional []int             `json:"optional,omitempty"`
	Labels   map[string]string `json:"labels"`
	Inner    *normInner        `json:"inner"`
	Nested   []normInner       `json:"nested"`
	When     time.Time         `json:"when"`
	Missing  *normInner        `json:"missing"`
}

func TestNormalizeJSONFillsRequiredNilSlices(t *testing.T) {
	in := &normOuter{Inner: &normInner{}, Nested: []normInner{{}}}
	raw, err := json.Marshal(normalizeJSON(in))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	if items, ok := got["items"].([]any); !ok || len(items) != 0 {
		t.Errorf("items = %v, want []", got["items"])
	}
	if _, ok := got["optional"]; ok {
		t.Error("omitempty field must stay omitted")
	}
	if _, ok := got["labels"].(map[string]any); !ok {
		t.Errorf("labels = %v, want {}", got["labels"])
	}
	if tags, ok := got["inner"].(map[string]any)["tags"].([]any); !ok || len(tags) != 0 {
		t.Errorf("inner.tags = %v, want []", got["inner"])
	}
	if tags := got["nested"].([]any)[0].(map[string]any)["tags"]; tags == nil {
		t.Error("nested[0].tags must be []")
	}
	if got["missing"] != nil {
		t.Error("nil pointers stay null")
	}
	if in.Items != nil || in.Inner.Tags != nil {
		t.Fatal("input was mutated")
	}
}
