package cli

import (
	"fmt"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func runDeploySteps(steps []deployStep) error {
	for i, step := range steps {
		logs.Info("[DEPLOY][%02d/%02d] %s", i+1, len(steps), step.name)
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return nil
}
