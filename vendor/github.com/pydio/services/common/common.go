package common

import "github.com/golang/protobuf/ptypes/any"

// Manager interface
type Manager interface {
	Get(string) interface{}
	Set(string, interface{}) error
	Del(string) error
}

// Converter interface
type Converter interface {
	Convert(*any.Any) (string, bool)
}
