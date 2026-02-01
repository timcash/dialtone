package app

import (
	"encoding/json"
)

// CADObject represents a parametric 3D object
type CADObject struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewGearObject creates a new parametric gear object matching gear_generator.py logic
func NewGearObject(outer float64, inner float64, teeth int) CADObject {
	return CADObject{
		Type: "gear",
		Parameters: map[string]interface{}{
			"outer_diameter":         outer,
			"inner_diameter":         inner,
			"num_teeth":              teeth,
			"thickness":              8.0,
			"tooth_height":           6.0,
			"tooth_width":            4.0,
			"num_mounting_holes":     4,
			"mounting_hole_diameter": 6.0,
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
