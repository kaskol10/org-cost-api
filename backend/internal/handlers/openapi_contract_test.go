package handlers

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// This test is the API contract check for third-party clients: real
// responses (demo data) must match the response schemas in openapi.yaml,
// and must not contain fields the spec doesn't document.

type schema struct {
	Ref                  string             `yaml:"$ref"`
	Type                 string             `yaml:"type"`
	Nullable             bool               `yaml:"nullable"`
	Enum                 []any              `yaml:"enum"`
	Properties           map[string]*schema `yaml:"properties"`
	Required             []string           `yaml:"required"`
	Items                *schema            `yaml:"items"`
	AdditionalProperties any                `yaml:"additionalProperties"`
	AllOf                []*schema          `yaml:"allOf"`
}

type openAPISpec struct {
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Schemas map[string]*schema `yaml:"schemas"`
	} `yaml:"components"`
}

type operation struct {
	Responses map[string]struct {
		Ref     string `yaml:"$ref"`
		Content map[string]struct {
			Schema *schema `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"responses"`
}

func loadSpec(t *testing.T) *openAPISpec {
	t.Helper()
	raw, err := os.ReadFile("../../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var s openAPISpec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return &s
}

type validator struct {
	spec *openAPISpec
	errs []string
}

// resolve follows $ref and single-element allOf wrappers (the OpenAPI 3.0
// way to mark a $ref nullable) to the schema that describes the value.
func (v *validator) resolve(s *schema) *schema {
	for s != nil {
		switch {
		case s.Ref != "":
			s = v.spec.Components.Schemas[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
		case len(s.AllOf) == 1 && s.Type == "" && len(s.Properties) == 0:
			s = s.AllOf[0]
		default:
			return s
		}
	}
	return nil
}

func (v *validator) fail(path, format string, args ...any) {
	v.errs = append(v.errs, path+": "+fmt.Sprintf(format, args...))
}

func (v *validator) check(path string, s *schema, val any) {
	wrapperNullable := s != nil && s.Nullable
	s = v.resolve(s)
	if s == nil {
		v.fail(path, "unresolvable schema")
		return
	}
	if val == nil {
		if !s.Nullable && !wrapperNullable && s.Type != "" {
			v.fail(path, "null but schema is %s (not nullable)", s.Type)
		}
		return
	}
	if len(s.AllOf) > 1 {
		for _, sub := range s.AllOf {
			v.check(path, sub, val)
		}
	}
	if len(s.Enum) > 0 {
		ok := false
		for _, e := range s.Enum {
			if fmt.Sprint(e) == fmt.Sprint(val) {
				ok = true
			}
		}
		if !ok {
			v.fail(path, "value %v not in enum %v", val, s.Enum)
		}
	}
	switch s.Type {
	case "string":
		if _, ok := val.(string); !ok {
			v.fail(path, "want string, got %T", val)
		}
	case "number":
		if _, ok := val.(float64); !ok {
			v.fail(path, "want number, got %T", val)
		}
	case "integer":
		f, ok := val.(float64)
		if !ok || f != float64(int64(f)) {
			v.fail(path, "want integer, got %v", val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			v.fail(path, "want boolean, got %T", val)
		}
	case "array":
		arr, ok := val.([]any)
		if !ok {
			v.fail(path, "want array, got %T", val)
			return
		}
		for i, item := range arr {
			v.check(fmt.Sprintf("%s[%d]", path, i), s.Items, item)
		}
	case "object", "":
		obj, ok := val.(map[string]any)
		if !ok {
			if s.Type == "object" {
				v.fail(path, "want object, got %T", val)
			}
			return
		}
		for _, req := range s.Required {
			if _, ok := obj[req]; !ok {
				v.fail(path, "missing required field %q", req)
			}
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := s.Properties[k]; ok {
				v.check(path+"."+k, ps, obj[k])
				continue
			}
			switch ap := s.AdditionalProperties.(type) {
			case bool:
				if !ap {
					v.fail(path, "undocumented field %q", k)
				}
			case map[string]any:
				// typed map values: re-decode the sub-schema
				raw, _ := yaml.Marshal(ap)
				var sub schema
				_ = yaml.Unmarshal(raw, &sub)
				v.check(path+"."+k, &sub, obj[k])
			default:
				// No additionalProperties: an object with declared properties
				// must not carry fields the spec doesn't document.
				if len(s.Properties) > 0 || len(s.AllOf) > 0 {
					v.fail(path, "undocumented field %q", k)
				}
			}
		}
	}
}

func TestResponsesMatchOpenAPISchemas(t *testing.T) {
	spec := loadSpec(t)
	h := demoAPI(t, "")

	cases := []struct {
		method, path, body string
	}{
		{"GET", "/api/v1/health", ""},
		{"GET", "/api/v1/ready", ""},
		{"GET", "/api/v1/meta", ""},
		{"GET", "/api/v1/accounts", ""},
		{"GET", "/api/v1/dashboard", ""},
		{"GET", "/api/v1/dashboard?period=mtd", ""},
		{"GET", "/api/v1/trends", ""},
		{"GET", "/api/v1/suggestions", ""},
		{"GET", "/api/v1/report", ""},
		{"GET", "/api/v1/report?view=lite", ""},
		{"GET", "/api/v1/report?period=mtd", ""},
		{"GET", "/api/v1/account-costs?account=production", ""},
		{"POST", "/api/v1/ask", `{"question":"What are our top services?"}`},
		{"POST", "/api/v1/ask", `{"question":"unattached ebs volumes"}`},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		route := c.path
		if i := strings.IndexByte(route, '?'); i >= 0 {
			route = route[:i]
		}
		op, ok := spec.Paths[route][strings.ToLower(c.method)]
		if !ok {
			t.Errorf("%s %s: not in spec", c.method, route)
			continue
		}
		resp, ok := op.Responses[fmt.Sprint(rec.Code)]
		if !ok {
			t.Errorf("%s %s: status %d not documented (body %s)", c.method, c.path, rec.Code, rec.Body.String())
			continue
		}
		if resp.Ref != "" {
			continue // shared error responses
		}
		media, ok := resp.Content["application/json"]
		if !ok || media.Schema == nil {
			t.Errorf("%s %s: no JSON schema for %d", c.method, c.path, rec.Code)
			continue
		}
		var body any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: invalid JSON: %v", c.method, c.path, err)
		}
		v := &validator{spec: spec}
		v.check("$", media.Schema, body)
		if len(v.errs) > 0 {
			t.Errorf("%s %s does not match openapi.yaml (%d problems):\n  %s",
				c.method, c.path, len(v.errs), strings.Join(dedupe(v.errs), "\n  "))
		}
	}
}

var indexRe = regexp.MustCompile(`\[\d+\]`)

// dedupe collapses array indexes so one drift isn't reported once per row.
func dedupe(errs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range errs {
		k := indexRe.ReplaceAllString(e, "[]")
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}
