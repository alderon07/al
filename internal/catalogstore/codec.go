package catalogstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

const MaxDocumentBytes = 8 << 20

func Encode(value any) ([]byte, error) {
	if err := Validate(value); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(value); err != nil {
		return nil, errors.New("cannot encode catalog state")
	}
	if b.Len() > MaxDocumentBytes {
		return nil, errors.New("catalog state exceeds size limit")
	}
	return b.Bytes(), nil
}
func Decode(data []byte, destination any) error {
	if len(data) > MaxDocumentBytes || !utf8.Valid(data) {
		return errors.New("invalid catalog state size or encoding")
	}
	t := reflect.TypeOf(destination)
	if t == nil || t.Kind() != reflect.Pointer || reflect.ValueOf(destination).IsNil() {
		return errors.New("catalog state destination must be a pointer")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := walk(d, 0, t.Elem(), map[reflect.Type]map[string]stateField{}); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing catalog state data")
	}
	v := reflect.New(t.Elem())
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v.Interface()); err != nil {
		return errors.New("invalid catalog state fields")
	}
	if err := Validate(v.Elem().Interface()); err != nil {
		return err
	}
	reflect.ValueOf(destination).Elem().Set(v.Elem())
	return nil
}

type stateField struct {
	typeOf   reflect.Type
	required bool
}

func stateFields(t reflect.Type, schemas map[reflect.Type]map[string]stateField) map[string]stateField {
	if fields, ok := schemas[t]; ok {
		return fields
	}
	fields := make(map[string]stateField, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			fields[name] = stateField{typeOf: field.Type, required: !strings.Contains(tag, ",omitempty")}
		}
	}
	schemas[t] = fields
	return fields
}

func walk(d *json.Decoder, depth int, t reflect.Type, schemas map[reflect.Type]map[string]stateField) error {
	if depth > 40 {
		return errors.New("catalog state nesting exceeds limit")
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	token, err := d.Token()
	if err != nil || token == nil {
		return errors.New("invalid catalog state JSON")
	}
	delim, container := token.(json.Delim)
	if !container {
		if t.Kind() == reflect.Struct || t.Kind() == reflect.Map || t.Kind() == reflect.Slice {
			return errors.New("invalid catalog state field shape")
		}
		return nil
	}
	switch delim {
	case '{':
		if t.Kind() != reflect.Struct && t.Kind() != reflect.Map {
			return errors.New("catalog state object required")
		}
		seen := map[string]bool{}
		var fields map[string]stateField
		if t.Kind() == reflect.Struct {
			fields = stateFields(t, schemas)
		}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return errors.New("invalid catalog state object")
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate catalog state field")
			}
			seen[name] = true
			var child reflect.Type
			if t.Kind() == reflect.Map {
				child = t.Elem()
			} else {
				field, known := fields[name]
				if !known {
					return errors.New("unknown catalog state field")
				}
				child = field.typeOf
			}
			if err := walk(d, depth+1, child, schemas); err != nil {
				return err
			}
		}
		for name, field := range fields {
			if field.required && !seen[name] {
				return errors.New("required catalog state field missing")
			}
		}
	case '[':
		if t.Kind() != reflect.Slice {
			return errors.New("catalog state array required")
		}
		for d.More() {
			if err := walk(d, depth+1, t.Elem(), schemas); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid catalog state delimiter")
	}
	end, err := d.Token()
	if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
		return errors.New("invalid catalog state delimiter")
	}
	return nil
}
