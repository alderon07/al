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
	if err := walk(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing catalog state data")
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.New("invalid catalog state JSON")
	}
	if err := required(raw, t.Elem()); err != nil {
		return err
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
func walk(d *json.Decoder, depth int) error {
	if depth > 40 {
		return errors.New("catalog state nesting exceeds limit")
	}
	token, err := d.Token()
	if err != nil || token == nil {
		return errors.New("invalid catalog state JSON")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return errors.New("invalid catalog state object")
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return errors.New("duplicate catalog state field")
			}
			seen[s] = true
			if err := walk(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := walk(d, depth+1); err != nil {
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
func required(raw any, t reflect.Type) error {
	if t.Kind() == reflect.Pointer {
		return required(raw, t.Elem())
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := raw.(map[string]any)
		if !ok {
			return errors.New("catalog state object required")
		}
		known := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
			if name != "" && name != "-" {
				known[name] = true
			}
		}
		for key := range obj {
			if !known[key] {
				return errors.New("unknown catalog state field")
			}
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			name := strings.Split(tag, ",")[0]
			if name == "" || name == "-" {
				continue
			}
			v, exists := obj[name]
			if !exists {
				if !strings.Contains(tag, ",omitempty") {
					return errors.New("required catalog state field missing")
				}
				continue
			}
			if err := required(v, f.Type); err != nil {
				return err
			}
		}
	case reflect.Slice:
		values, ok := raw.([]any)
		if !ok {
			return errors.New("catalog state array required")
		}
		for _, v := range values {
			if err := required(v, t.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		values, ok := raw.(map[string]any)
		if !ok {
			return errors.New("catalog state map required")
		}
		for _, v := range values {
			if err := required(v, t.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
