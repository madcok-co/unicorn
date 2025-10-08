// ============================================
// UNICORN Framework - Struct Binding System
// Auto-bind URL params, query params, and body to struct
// ============================================

package unicorn

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	// ErrInvalidBindTarget indicates the bind target is not a pointer to struct
	ErrInvalidBindTarget = errors.New("bind target must be a pointer to struct")

	// ErrFieldNotFound indicates the field was not found in the struct
	ErrFieldNotFound = errors.New("field not found")
)

// Binder handles binding request data to structs.
type Binder struct{}

// NewBinder creates a new binder instance.
func NewBinder() *Binder {
	return &Binder{}
}

// Bind binds request data to a struct.
// It supports URL params, query params, and JSON body.
// Use struct tags: `bind:"field_name"`
func (b *Binder) Bind(data map[string]interface{}, target interface{}) error {
	// Validate target
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return ErrInvalidBindTarget
	}

	targetValue = targetValue.Elem()
	if targetValue.Kind() != reflect.Struct {
		return ErrInvalidBindTarget
	}

	targetType := targetValue.Type()

	// Iterate through struct fields
	for i := 0; i < targetValue.NumField(); i++ {
		field := targetValue.Field(i)
		fieldType := targetType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get bind tag
		tag := fieldType.Tag.Get("bind")
		if tag == "" {
			tag = strings.ToLower(fieldType.Name)
		}

		// Get value from data
		value, ok := data[tag]
		if !ok {
			continue
		}

		// Set field value
		if err := b.setField(field, value); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setField sets a struct field value with type conversion.
func (b *Binder) setField(field reflect.Value, value interface{}) error {
	if value == nil {
		return nil
	}

	// Get field kind
	kind := field.Kind()

	// Handle based on field type
	switch kind {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", value))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := b.toInt64(value)
		if err != nil {
			return err
		}
		field.SetInt(intVal)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := b.toUint64(value)
		if err != nil {
			return err
		}
		field.SetUint(uintVal)

	case reflect.Float32, reflect.Float64:
		floatVal, err := b.toFloat64(value)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)

	case reflect.Bool:
		boolVal, err := b.toBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)

	case reflect.Slice:
		return b.setSliceField(field, value)

	case reflect.Map:
		return b.setMapField(field, value)

	case reflect.Struct:
		return b.setStructField(field, value)

	default:
		return fmt.Errorf("unsupported field type: %s", kind)
	}

	return nil
}

// Type conversion helpers

func (b *Binder) toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

func (b *Binder) toUint64(value interface{}) (uint64, error) {
	switch v := value.(type) {
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case int:
		return uint64(v), nil
	case int8:
		return uint64(v), nil
	case int16:
		return uint64(v), nil
	case int32:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case float32:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	case string:
		return strconv.ParseUint(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", value)
	}
}

func (b *Binder) toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

func (b *Binder) toBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func (b *Binder) setSliceField(field reflect.Value, value interface{}) error {
	// Convert value to slice
	valueSlice, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("value is not a slice")
	}

	// Create new slice
	sliceType := field.Type()
	newSlice := reflect.MakeSlice(sliceType, len(valueSlice), len(valueSlice))

	// Set each element
	for i, item := range valueSlice {
		elem := newSlice.Index(i)
		if err := b.setField(elem, item); err != nil {
			return err
		}
	}

	field.Set(newSlice)
	return nil
}

func (b *Binder) setMapField(field reflect.Value, value interface{}) error {
	// Convert value to map
	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("value is not a map")
	}

	// Create new map
	mapType := field.Type()
	newMap := reflect.MakeMap(mapType)

	// Set each key-value pair
	for k, v := range valueMap {
		keyVal := reflect.ValueOf(k)
		elemVal := reflect.New(mapType.Elem()).Elem()

		if err := b.setField(elemVal, v); err != nil {
			return err
		}

		newMap.SetMapIndex(keyVal, elemVal)
	}

	field.Set(newMap)
	return nil
}

func (b *Binder) setStructField(field reflect.Value, value interface{}) error {
	// Try to convert to JSON and unmarshal
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, field.Addr().Interface())
}

// Global binder instance
var globalBinder = NewBinder()

// Bind binds request data to a struct using the global binder.
func Bind(data map[string]interface{}, target interface{}) error {
	return globalBinder.Bind(data, target)
}
