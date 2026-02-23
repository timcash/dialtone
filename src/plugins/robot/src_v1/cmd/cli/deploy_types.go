package cli

type deployOptions struct {
	Host      string
	Port      string
	User      string
	Pass      string
	Ephemeral bool
	Relay     bool
	Service   bool
	SmokeTest bool
}

type deployContext struct {
	versionDir string
	opts       deployOptions
	repoRoot   string
	localBin   string
	goos       string
	goarch     string

	remoteRoot   string
	remoteBinDir string
	remoteUIDir  string
	remoteBin    string
	remoteBinTmp string
}

type deployStep struct {
	name string
	run  func() error
}
