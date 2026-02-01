package test

import (
	"fmt"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/plugins/cad/app"
)

func init() {
	test.Register("cad-object-creation", "cad", []string{"plugin", "cad"}, RunCADObjectTest)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running cad plugin suite...")
	return test.RunPlugin("cad")
}

func RunCADObjectTest() error {
	gear := app.NewGearObject(80.0, 20.0, 20)
	if gear.Type != "gear" {
		return fmt.Errorf("unexpected gear type: %s", gear.Type)
	}
	if gear.Parameters["num_teeth"] != 20 {
		return fmt.Errorf("unexpected teeth count: %v", gear.Parameters["num_teeth"])
	}
	if gear.Parameters["outer_diameter"] != 80.0 {
		return fmt.Errorf("unexpected outer diameter: %v", gear.Parameters["outer_diameter"])
	}
	fmt.Println("PASS: [cad] Plugin logic verified")
	return nil
}
