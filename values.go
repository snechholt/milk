package milk

type Values map[interface{}]interface{}

// Get returns the given key's value from the request path parameters or querystring.
// The request path is searched first, and overrides any querystring values with the same key.
func (this Values) Get(key interface{}) interface{} {
	return this[key]
}

func (this Values) GetString(key interface{}) string {
	if val, ok := this[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt64 returns the given key's value as an int.
// Returns 0 for invalid or missing values.
func (this Values) GetInt(key interface{}) int {
	if val, ok := this[key]; ok {
		if v, ok := val.(int); ok {
			return v
		}
	}
	return 0
}

// GetInt64 returns the given key's value as an int64.
// Returns 0 for invalid or missing values.
func (this Values) GetInt64(key interface{}) int64 {
	if val, ok := this[key]; ok {
		if v, ok := val.(int64); ok {
			return v
		}
	}
	return 0
}

func (this Values) Set(key interface{}, val interface{}) {
	this[key] = val
}
