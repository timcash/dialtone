package app

import (
	"encoding/json"
)

// CADObject represents a parametric 3D object
type CADObject struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewGearObject creates a new parametric gear object
func NewGearObject(teeth int, diameter float64, thickness float64) CADObject {
	return CADObject{
		Type: "gear",
		Parameters: map[string]interface{}{
			"teeth":     teeth,
			"diameter":  diameter,
			"thickness": thickness,
		},
	}
}

// ToJSON converts the CAD object to a JSON string
func (o CADObject) ToJSON() (string, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
