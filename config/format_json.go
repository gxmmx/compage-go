package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type jsonFormat struct{}

func (jsonFormat) Decode(data []byte) (map[string]any, error) { return decodeJSONFormat(data) }
func (jsonFormat) Encode(root map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(root, "", "  ")
	return append(data, '\n'), err
}

func decodeJSONFormat(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeJSONValueFormat(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON value")
	}
	root, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("top-level JSON value must be an object")
	}
	return root, nil
}

func decodeJSONValueFormat(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			rawKey, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := rawKey.(string)
			if !ok {
				return nil, fmt.Errorf("object key is not a string")
			}
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("duplicate key %q", key)
			}
			value, err := decodeJSONValueFormat(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("unterminated object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeJSONValueFormat(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("unterminated array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}
