package test

import "strings"

type Options struct {
	AttachNode string
	TargetURL  string
}

var suiteOptions Options

func SetOptions(opts Options) {
	suiteOptions = Options{
		AttachNode: strings.TrimSpace(opts.AttachNode),
		TargetURL:  strings.TrimSpace(opts.TargetURL),
	}
}

func GetOptions() Options {
	return suiteOptions
}
