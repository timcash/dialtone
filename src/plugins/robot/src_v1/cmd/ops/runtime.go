package ops

import (
	robotv1 "dialtone/dev/plugins/robot/src_v1/go"
)

func resolveRobotPathsPreset() (robotv1.Paths, error) {
	return robotv1.ResolvePaths("")
}
