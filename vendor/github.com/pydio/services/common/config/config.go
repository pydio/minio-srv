package config

// Map structure to store configuration
type Map map[string]interface{}

// NewMap variable
func NewMap() *Map {
	return &Map{}
}

// Get gets the first value associated with the given key.
// If there are no values associated with the key, Get returns
// the empty string. To access multiple values, use the map
// directly.
func (c Map) Get(key string) interface{} {
	if c == nil {
		return nil
	}
	if v, ok := c[key]; ok {
		return v
	}
	return nil
}

// Set sets the key to value. It replaces any existing
// values.
func (c Map) Set(key string, value interface{}) error {
	c[key] = value

	return nil
}

// Del deletes the values associated with key.
func (c Map) Del(key string) error {
	delete(c, key)

	return nil
}
