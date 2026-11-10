package handlers

import (
	"encoding/json"
	"reflect"
	"strings"
)

// normalizeJSON returns a copy of v in which nil slices and maps that would
// be encoded (no omitempty) become empty, so clients get [] / {} instead of
// null for fields openapi.yaml marks required. The input is never mutated:
// responses are often shared cache values.
func normalizeJSON(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	out := normalizeValue(rv, true)
	if !out.IsValid() {
		return v
	}
	return out.Interface()
}

var jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()

func normalizeValue(v reflect.Value, fillNil bool) reflect.Value {
	if !v.IsValid() {
		return v
	}
	t := v.Type()
	if t.Implements(jsonMarshalerType) || (t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(jsonMarshalerType)) {
		return v // custom encoding (time.Time etc.): leave as is
	}
	switch t.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		elem := normalizeValue(v.Elem(), fillNil)
		p := reflect.New(t.Elem())
		p.Elem().Set(elem)
		return p
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		inner := normalizeValue(v.Elem(), fillNil)
		out := reflect.New(t).Elem()
		out.Set(inner)
		return out
	case reflect.Slice:
		if v.IsNil() {
			if fillNil && t.Elem().Kind() != reflect.Uint8 {
				return reflect.MakeSlice(t, 0, 0)
			}
			return v
		}
		out := reflect.MakeSlice(t, v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(normalizeValue(v.Index(i), true))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			if fillNil {
				return reflect.MakeMap(t)
			}
			return v
		}
		out := reflect.MakeMapWithSize(t, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), normalizeValue(iter.Value(), true))
		}
		return out
	case reflect.Struct:
		out := reflect.New(t).Elem()
		out.Set(v)
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			omit := strings.Contains(tag, ",omitempty") || strings.Contains(tag, ",omitzero")
			out.Field(i).Set(normalizeValue(v.Field(i), !omit))
		}
		return out
	default:
		return v
	}
}
