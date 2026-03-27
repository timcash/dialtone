package support

import (
	"os"
	"strings"
)

type SSHFixture struct {
	Alias string
	Host  string
	User  string
	Port  string
}

func ResolveSSHFixture() SSHFixture {
	alias := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_NAME"))
	if alias == "" {
		alias = "wsl"
	}
	host := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_HOST"))
	if host == "" {
		host = "grey.shad-artichoke.ts.net"
	}
	user := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_USER"))
	if user == "" {
		user = "user"
	}
	port := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_PORT"))
	if port == "" {
		port = "22"
	}
	return SSHFixture{
		Alias: alias,
		Host:  host,
		User:  user,
		Port:  port,
	}
}
