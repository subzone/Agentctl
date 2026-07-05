package controlplane

import (
	"encoding/json"
	"fmt"
)

// flexString unmarshals a JSON string or number into a string field.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	return fmt.Errorf("flexString: unsupported JSON: %s", string(b))
}

func (f flexString) String() string { return string(f) }
