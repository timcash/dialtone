package test

import "strings"

type Options struct {
	AttachNode      string
	TargetURL       string
	ClicksPerSecond float64
}

var suiteOptions Options

func SetOptions(opts Options) {
	suiteOptions = Options{
		AttachNode:      strings.TrimSpace(opts.AttachNode),
		TargetURL:       strings.TrimSpace(opts.TargetURL),
		ClicksPerSecond: opts.ClicksPerSecond,
	}
}

func GetOptions() Options {
	return suiteOptions
}
