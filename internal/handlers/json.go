package handlers

import (
	"encoding/json"

	"gorm.io/datatypes"
)

// rawJSON decodes a datatypes.JSON column into a generic value for response
// embedding. Returns nil when empty/invalid (matches Laravel's null cast).
func rawJSON(j datatypes.JSON) interface{} {
	if len(j) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(j, &v); err != nil {
		return nil
	}
	return v
}

// rawJSONString decodes a JSON string column (e.g. notifications.data) into a
// generic value. Returns nil when empty/invalid.
func rawJSONString(s string) interface{} {
	if s == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

// decodeJSON unmarshals a datatypes.JSON column into the given target.
func decodeJSON(j datatypes.JSON, target interface{}) {
	if len(j) == 0 {
		return
	}
	_ = json.Unmarshal(j, target)
}
