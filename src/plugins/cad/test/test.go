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
	gear := app.NewGearObject(12, 5.0, 1.0)
	if gear.Type != "gear" {
		return fmt.Errorf("unexpected gear type: %s", gear.Type)
	}
	if gear.Parameters["teeth"] != 12 {
		return fmt.Errorf("unexpected teeth count: %v", gear.Parameters["teeth"])
	}
	fmt.Println("PASS: [cad] Plugin logic verified")
	return nil
}
