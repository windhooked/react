// Copyright 2018-20 PJ Engineering and Business Solutions Pty. Ltd. All rights reserved.

package react

import (
	"errors"
	"reflect"
	"strings"

	"github.com/gopherjs/gopherjs/js"
	"github.com/rocketlaunchr/react/forks/mapstructure"
)

// SToMap will convert a struct or pass-through a map.
// If the argument is a struct, it will convert it to a map.
// If the argument is a map, it will pass it through.
// If the argument is nil, it will return nil.
func SToMap(s interface{}) map[string]interface{} {

	if s == nil {
		return nil
	}

	if jsObjectIsNil(s) {
		return nil
	}

	// Check if s is a struct
	if isStruct(s) {
		return convertStruct(s)
	}

	switch x := s.(type) {
	case js.M:
		return map[string]interface{}(x)
	case map[string]interface{}:
		return x
	default:
		s := reflect.ValueOf(x)
		if s.IsNil() {
			return nil
		}
		panic("unrecognized type")
	}
}

// jsObjectIsNotNil returns true if x is a js object
// and is not null.
func jsObjectIsNotNil(x interface{}) bool {
	if v, ok := x.(*js.Object); !ok || v == nil {
		return false
	}

	return true
}

// jsObjectIsNil return true if x is a js object and is null.
func jsObjectIsNil(x interface{}) bool {
	if v, ok := x.(*js.Object); ok && v == nil {
		return true
	}
	return false
}

// convertStruct will convert a struct into a map.
func convertStruct(sIn interface{}) map[string]interface{} {

	out := map[string]interface{}{}

	s := reflect.ValueOf(sIn)

	// Check if s is a pointer
	if s.Kind() == reflect.Ptr {
		s = reflect.Indirect(s)
	}
	typeOfT := s.Type()

	for i := 0; i < s.NumField(); i++ {
		f := typeOfT.Field(i)

		if f.PkgPath != "" {
			// not exported
			continue
		}

		fieldName := typeOfT.Field(i).Name
		fieldTag := f.Tag.Get("react")
		fieldValRaw := s.Field(i)
		fieldVal := fieldValRaw.Interface()

		if fieldTag == "-" || (!jsObjectIsNotNil(fieldVal) && strings.HasSuffix(fieldTag, ",omitempty") && (fieldVal == nil || jsObjectIsNil(fieldVal) || reflect.DeepEqual(fieldVal, reflect.Zero(reflect.TypeOf(fieldVal)).Interface()))) {
			// Omit field
			continue
		}

		// Deal with Sets as a special case
		if set, ok := fieldVal.(Set); ok {
			base := strings.TrimSuffix(fieldTag, ",omitempty")
			if strings.TrimSpace(base) == "" {
				// Skip this Set
				continue
			}

			all := set.Convert(base)
			for attr, val := range all {
				out[attr] = val
			}
			continue
		}

		// Deal with dangerouslySetInnerHTML as a special case
		if fieldName == "DangerouslySetInnerHTML" && strings.TrimSuffix(fieldTag, ",omitempty") == "dangerouslySetInnerHTML" {
			if fn, ok := fieldVal.(func() interface{}); ok {
				mp := DangerouslySetInnerHTMLFunc(fn)
				out["dangerouslySetInnerHTML"] = mp["dangerouslySetInnerHTML"]
			} else {
				mp := DangerouslySetInnerHTML(fieldVal)
				out["dangerouslySetInnerHTML"] = mp["dangerouslySetInnerHTML"]
			}
			continue
		}

		// Deal with slices as a special case
		if fieldValRaw.Kind() == reflect.Slice {
			slc := []interface{}{}
			for i := 0; i < fieldValRaw.Len(); i++ {
				e := fieldValRaw.Index(i)
				slc = append(slc, convertStruct(e.Interface()))
			}

			if fieldTag == "" {
				out[fieldName] = slc
			} else {
				out[strings.TrimSuffix(fieldTag, ",omitempty")] = slc
			}
			continue
		}

		if fieldTag == "" {
			if jsObjectIsNotNil(fieldVal) {
				out[fieldName] = fieldVal
			} else if isStruct(fieldVal) {
				out[fieldName] = convertStruct(fieldVal)
			} else {
				out[fieldName] = fieldVal
			}
		} else {
			if jsObjectIsNotNil(fieldVal) {
				out[strings.TrimSuffix(fieldTag, ",omitempty")] = fieldVal
			} else if isStruct(fieldVal) {
				out[strings.TrimSuffix(fieldTag, ",omitempty")] = convertStruct(fieldVal)
			} else {
				out[strings.TrimSuffix(fieldTag, ",omitempty")] = fieldVal
			}
		}
	}

	return out
}

// isStruct returns true if s is a struct.
func isStruct(s interface{}) bool {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// uninitialized zero value of a struct
	if v.Kind() == reflect.Invalid {
		return false
	}

	return v.Kind() == reflect.Struct
}

// UnmarshalStruct will unmarshal a struct with values from a map.
// strct must be a pointer to a struct. Use struct tag "react" for linking
// map keys to the struct's fields.
func UnmarshalStruct(mp map[string]interface{}, strct interface{}) error {

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ZeroFields: true,
		TagName:    "react",
		Result:     strct,
	})
	if err != nil {
		panic(err)
	}

	return decoder.Decode(mp)
}

// UnmarshalProps will unmarshal a given struct with values from
// the component's prop. strct must be a pointer to a struct.
func UnmarshalProps(this *js.Object, strct interface{}) error {
	props := this.Get("props").Interface().(map[string]interface{})
	return UnmarshalStruct(props, strct)
}

// UnmarshalState will unmarshal a given struct with values from
// the component's state. strct must be a pointer to a struct.
func UnmarshalState(this *js.Object, strct interface{}) error {
	state := this.Get("state").Interface().(map[string]interface{})
	return UnmarshalStruct(state, strct)
}

// HydrateProps will hydrate a given struct with values from
// the component's prop. strct must be a pointer to a struct.
//
// Deprecated: Use UnmarshalProps instead.
func HydrateProps(this *js.Object, strct interface{}) error {
	return UnmarshalProps(this, strct)
}

// HydrateState will hydrate a given struct with values from
// the component's state. strct must be a pointer to a struct.
//
// Deprecated: Use UnmarshalState instead.
func HydrateState(this *js.Object, strct interface{}) error {
	return UnmarshalState(this, strct)
}

// JSONUnmarshal provides a simple way to unmarshal json encoded strings to structs.
//
// See: https://github.com/gopherjs/gopherjs/wiki/Using-native-JSON-parsing-to-realize-a-slim-JSON-decoder
// for a tutorial with an example.
func JSONUnmarshal(json string) (*js.Object, error) {

	obj, err := JSFn("JSON.parse", json)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, errors.New("JSONUnmarshal: something went wrong.")
	}

	return obj, nil
}
