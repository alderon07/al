package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errShadowDuplicateConfigField = errors.New("duplicate configuration field")

func decodeUniqueJSON(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return fmt.Errorf("JSON is too deeply nested")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("invalid object field")
				}
				if seen[key] {
					return errShadowDuplicateConfigField
				}
				seen[key] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	strict := json.NewDecoder(bytes.NewReader(contents))
	strict.DisallowUnknownFields()
	return strict.Decode(target)
}
