package handlers

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

// Exhaustive spec check: walks each endpoint's Go response type by
// reflection alongside its schema in openapi.yaml and requires the same
// fields, compatible types, and matching "required" (non-omitempty) sets.
// Unlike the runtime contract test this covers fields demo data leaves empty.
//
// Authoring aid: OPENAPI_PRINT_SCHEMAS=1 go test ./internal/handlers -run Parity
// prints a schema skeleton for every reachable Go type.

var parityRoots = map[string]reflect.Type{
	"/api/v1/ready":              reflect.TypeOf(service.ReadyStatus{}),
	"/api/v1/dashboard":          reflect.TypeOf(service.DashboardResponse{}),
	"/api/v1/trends":             reflect.TypeOf(analysis.TrendsResponse{}),
	"/api/v1/suggestions":        reflect.TypeOf(analysis.SuggestionsResponse{}),
	"/api/v1/report":             reflect.TypeOf(service.ReportResponse{}),
	"/api/v1/account-costs":      reflect.TypeOf(service.AccountCostsResponse{}),
	"/api/v1/ask":                reflect.TypeOf(service.AskResponse{}),
	"/api/v1/service-detail":     reflect.TypeOf(costexplorer.ServiceDetail{}),
	"/api/v1/service-tag-delta":  reflect.TypeOf(analysis.ServiceTagDeltaResponse{}),
	"/api/v1/service-tag-totals": reflect.TypeOf(analysis.ServiceTagTotalsResponse{}),
}

var timeType = reflect.TypeOf(time.Time{})

type jsonField struct {
	name      string
	typ       reflect.Type
	omitempty bool
}

func jsonFields(t reflect.Type) []jsonField {
	var out []jsonField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if f.Anonymous && name == "" {
			et := f.Type
			if et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct {
				out = append(out, jsonFields(et)...)
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		omit := false
		for _, p := range parts[1:] {
			if p == "omitempty" || p == "omitzero" {
				omit = true
			}
		}
		out = append(out, jsonField{name: name, typ: f.Type, omitempty: omit})
	}
	return out
}

type parity struct {
	v    *validator
	errs []string
	seen map[string]bool
}

func (p *parity) fail(path, format string, args ...any) {
	p.errs = append(p.errs, path+": "+fmt.Sprintf(format, args...))
}

func (p *parity) compare(path string, t reflect.Type, s *schema) {
	s = p.v.resolve(s)
	if s == nil {
		p.fail(path, "no schema")
		return
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	key := t.String() + "|" + fmt.Sprintf("%p", s)
	if p.seen[key] {
		return
	}
	p.seen[key] = true

	if t == timeType {
		if s.Type != "string" {
			p.fail(path, "time.Time must be type string, spec has %q", s.Type)
		}
		return
	}
	switch t.Kind() {
	case reflect.String:
		p.want(path, s, "string")
	case reflect.Bool:
		p.want(path, s, "boolean")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		p.want(path, s, "integer")
	case reflect.Float32, reflect.Float64:
		if s.Type != "number" && s.Type != "integer" {
			p.fail(path, "want number, spec has %q", s.Type)
		}
	case reflect.Interface:
		// any: spec may say anything
	case reflect.Slice, reflect.Array:
		if !p.want(path, s, "array") {
			return
		}
		if s.Items == nil {
			p.fail(path, "array without items")
			return
		}
		p.compare(path+"[]", t.Elem(), s.Items)
	case reflect.Map:
		if s.Type != "object" {
			p.fail(path, "map must be type object, spec has %q", s.Type)
			return
		}
		switch ap := s.AdditionalProperties.(type) {
		case bool:
			if !ap {
				p.fail(path, "map needs additionalProperties")
			}
		case nil:
			p.fail(path, "map needs additionalProperties (true or a schema)")
		}
	case reflect.Struct:
		if s.Type != "object" {
			p.fail(path, "struct %s must be type object, spec has %q", t.Name(), s.Type)
			return
		}
		goFields := jsonFields(t)
		goNames := map[string]bool{}
		for _, f := range goFields {
			goNames[f.name] = true
			ps, ok := s.Properties[f.name]
			if !ok {
				p.fail(path, "field %q (%s) missing from spec", f.name, f.typ)
				continue
			}
			isReq := false
			for _, r := range s.Required {
				if r == f.name {
					isReq = true
				}
			}
			wantReq := !f.omitempty
			if isReq != wantReq {
				p.fail(path, "field %q: required=%v in spec, but Go %s (want required=%v)", f.name, isReq, map[bool]string{true: "has omitempty", false: "always sends it"}[f.omitempty], wantReq)
			}
			nullable := f.typ.Kind() == reflect.Pointer && !f.omitempty
			if ps.Nullable != nullable {
				if ps.Ref != "" && nullable {
					p.fail(path, "field %q can be null: wrap the $ref as nullable: true + allOf", f.name)
				} else if ps.Ref == "" {
					p.fail(path, "field %q: nullable=%v in spec, want %v", f.name, ps.Nullable, nullable)
				}
			}
			p.compare(path+"."+f.name, f.typ, ps)
		}
		var extra []string
		for name := range s.Properties {
			if !goNames[name] {
				extra = append(extra, name)
			}
		}
		sort.Strings(extra)
		for _, name := range extra {
			p.fail(path, "spec documents %q but the Go response has no such field", name)
		}
	default:
		p.fail(path, "unsupported Go kind %s", t.Kind())
	}
}

func (p *parity) want(path string, s *schema, typ string) bool {
	if s.Type != typ {
		p.fail(path, "want type %s, spec has %q", typ, s.Type)
		return false
	}
	return true
}

func TestOpenAPIParityWithGoTypes(t *testing.T) {
	spec := loadSpec(t)
	if os.Getenv("OPENAPI_PRINT_SCHEMAS") != "" {
		printSchemas(t)
	}
	paths := make([]string, 0, len(parityRoots))
	for p := range parityRoots {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		ops := spec.Paths[path]
		var op operation
		for _, m := range []string{"get", "post"} {
			if o, ok := ops[m]; ok {
				op = o
			}
		}
		resp, ok := op.Responses["200"]
		if !ok {
			t.Errorf("%s: no 200 response in spec", path)
			continue
		}
		media, ok := resp.Content["application/json"]
		if !ok || media.Schema == nil {
			t.Errorf("%s: no JSON schema for 200", path)
			continue
		}
		p := &parity{v: &validator{spec: spec}, seen: map[string]bool{}}
		p.compare("$", parityRoots[path], media.Schema)
		if len(p.errs) > 0 {
			t.Errorf("%s: openapi.yaml differs from %s (%d):\n  %s", path, parityRoots[path], len(p.errs), strings.Join(p.errs, "\n  "))
		}
	}
}

// printSchemas emits YAML schema skeletons for every Go type reachable
// from the roots, to paste into openapi.yaml (then add descriptions).
func printSchemas(t *testing.T) {
	done := map[reflect.Type]bool{}
	var queue []reflect.Type
	for _, r := range parityRoots {
		queue = append(queue, r)
	}
	var b strings.Builder
	var typeOf func(reflect.Type, string, bool)
	typeOf = func(ft reflect.Type, indent string, nullable bool) {
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		null := ""
		if nullable {
			null = "\n" + indent + "nullable: true"
		}
		switch {
		case ft == timeType:
			fmt.Fprintf(&b, "%stype: string\n%sformat: date-time%s\n", indent, indent, null)
		case ft.Kind() == reflect.Struct:
			if nullable {
				fmt.Fprintf(&b, "%snullable: true\n%sallOf:\n%s  - $ref: \"#/components/schemas/%s\"\n", indent, indent, indent, ft.Name())
			} else {
				fmt.Fprintf(&b, "%s$ref: \"#/components/schemas/%s\"\n", indent, ft.Name())
			}
			queue = append(queue, ft)
		case ft.Kind() == reflect.Slice:
			fmt.Fprintf(&b, "%stype: array%s\n%sitems:\n", indent, null, indent)
			typeOf(ft.Elem(), indent+"  ", false)
		case ft.Kind() == reflect.Map:
			fmt.Fprintf(&b, "%stype: object%s\n%sadditionalProperties:\n", indent, null, indent)
			typeOf(ft.Elem(), indent+"  ", false)
		case ft.Kind() == reflect.String:
			fmt.Fprintf(&b, "%stype: string%s\n", indent, null)
		case ft.Kind() == reflect.Bool:
			fmt.Fprintf(&b, "%stype: boolean%s\n", indent, null)
		case ft.Kind() == reflect.Float32 || ft.Kind() == reflect.Float64:
			fmt.Fprintf(&b, "%stype: number%s\n", indent, null)
		case ft.Kind() >= reflect.Int && ft.Kind() <= reflect.Uint64:
			fmt.Fprintf(&b, "%stype: integer%s\n", indent, null)
		default:
			fmt.Fprintf(&b, "%s{}\n", indent)
		}
	}
	for len(queue) > 0 {
		tt := queue[0]
		queue = queue[1:]
		if done[tt] {
			continue
		}
		done[tt] = true
		fmt.Fprintf(&b, "    %s: # %s\n      type: object\n", tt.Name(), tt)
		var req []string
		fields := jsonFields(tt)
		for _, f := range fields {
			if !f.omitempty {
				req = append(req, f.name)
			}
		}
		if len(req) > 0 {
			fmt.Fprintf(&b, "      required: [%s]\n", strings.Join(req, ", "))
		}
		b.WriteString("      properties:\n")
		for _, f := range fields {
			fmt.Fprintf(&b, "        %s:\n", f.name)
			typeOf(f.typ, "          ", f.typ.Kind() == reflect.Pointer && !f.omitempty)
		}
	}
	t.Log("\n" + b.String())
}
