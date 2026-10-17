package lesson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// decodeStrict は必須キー、正確なキー名、重複キー、nullを構造体への変換前に検査する。
func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := readJSON(decoder, 0)
	if err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	if err := checkShape(value, reflect.TypeOf(target).Elem()); err != nil {
		return err
	}
	value, err = normalizeNumbers(value)
	if err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(canonical, target)
}

// 全数値フィールドが整数のため、表記に依存せず整数性を検査する。
// float64への変換は丸めで小数を受理するおそれがあるため使用しない。
func normalizeNumbers(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			normalized, err := normalizeNumbers(item)
			if err != nil {
				return nil, err
			}
			typed[key] = normalized
		}
	case []any:
		for index, item := range typed {
			normalized, err := normalizeNumbers(item)
			if err != nil {
				return nil, err
			}
			typed[index] = normalized
		}
	case json.Number:
		return normalizeInteger(typed)
	}
	return value, nil
}

func normalizeInteger(value json.Number) (json.Number, error) {
	coefficient := string(value)
	sign := ""
	if strings.HasPrefix(coefficient, "-") {
		sign, coefficient = "-", coefficient[1:]
	}
	exponentText := "0"
	if index := strings.IndexAny(coefficient, "eE"); index >= 0 {
		exponentText, coefficient = coefficient[index+1:], coefficient[:index]
	}
	fractionDigits := 0
	if index := strings.IndexByte(coefficient, '.'); index >= 0 {
		fractionDigits = len(coefficient) - index - 1
		coefficient = coefficient[:index] + coefficient[index+1:]
	}
	coefficient = strings.TrimLeft(coefficient, "0")
	if coefficient == "" {
		return json.Number("0"), nil
	}
	exponent, err := strconv.ParseInt(exponentText, 10, 32)
	if err != nil {
		return "", errors.New("number is outside integer range")
	}
	scale := exponent - int64(fractionDigits)
	if scale < 0 {
		remove := -scale
		if remove > int64(len(coefficient)) || strings.TrimRight(coefficient[len(coefficient)-int(remove):], "0") != "" {
			return "", errors.New("expected integer")
		}
		coefficient = coefficient[:len(coefficient)-int(remove)]
	} else {
		if int64(len(coefficient))+scale > 19 {
			return "", errors.New("number is outside integer range")
		}
		coefficient += strings.Repeat("0", int(scale))
	}
	number, err := strconv.ParseInt(sign+coefficient, 10, strconv.IntSize)
	if err != nil {
		return "", errors.New("number is outside integer range")
	}
	return json.Number(strconv.FormatInt(number, 10)), nil
}

func readJSON(decoder *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("JSON nesting limit exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, errors.New("invalid object key")
			}
			if _, exists := object[name]; exists {
				return nil, fmt.Errorf("duplicate key %q", name)
			}
			value, err := readJSON(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		_, err := decoder.Token()
		return object, err
	case json.Delim('['):
		values := []any{}
		for decoder.More() {
			value, err := readJSON(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		_, err := decoder.Token()
		return values, err
	default:
		return token, nil
	}
}

func checkShape(value any, typ reflect.Type) error {
	if value == nil {
		return errors.New("null is not allowed")
	}
	switch typ.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return errors.New("expected object")
		}
		fields := make(map[string]bool)
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			tag := strings.Split(field.Tag.Get("json"), ",")
			name := tag[0]
			fields[name] = true
			item, exists := object[name]
			if !exists && len(tag) > 1 && tag[1] == "omitempty" {
				continue
			}
			if !exists {
				return fmt.Errorf("missing field %q", name)
			}
			if len(tag) > 1 && tag[1] == "omitempty" && field.Type.Kind() == reflect.String && item == "" {
				return fmt.Errorf("empty optional reference %q", name)
			}
			if err := checkShape(item, field.Type); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		for key := range object {
			if !fields[key] {
				return fmt.Errorf("unknown field %q", key)
			}
		}
	case reflect.Slice:
		items, ok := value.([]any)
		if !ok {
			return errors.New("expected array")
		}
		for _, item := range items {
			if err := checkShape(item, typ.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		items, ok := value.(map[string]any)
		if !ok {
			return errors.New("expected object")
		}
		for _, item := range items {
			if err := checkShape(item, typ.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}
