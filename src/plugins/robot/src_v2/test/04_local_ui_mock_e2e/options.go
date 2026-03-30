package localuimocke2e

import "strings"

type Options struct {
	BrowserBaseURL string
}

var suiteOptions Options

func SetOptions(opts Options) {
	suiteOptions = Options{
		BrowserBaseURL: strings.TrimSpace(opts.BrowserBaseURL),
	}
}

func GetOptions() Options {
	return suiteOptions
}
