package repl

import "time"

type busFrame struct {
	Type      string `json:"type"`
	From      string `json:"from,omitempty"`
	Room      string `json:"room,omitempty"`
	Version   string `json:"version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type dialtoneConfig struct {
	DialtoneEnv      string     `json:"DIALTONE_ENV,omitempty"`
	DialtoneRepoRoot string     `json:"DIALTONE_REPO_ROOT,omitempty"`
	DialtoneUseNix   string     `json:"DIALTONE_USE_NIX,omitempty"`
	MeshNodes        []meshNode `json:"mesh_nodes,omitempty"`
}

type meshNode struct {
	Name                string   `json:"name"`
	Aliases             []string `json:"aliases,omitempty"`
	User                string   `json:"user"`
	Host                string   `json:"host"`
	HostCandidates      []string `json:"host_candidates,omitempty"`
	RoutePreference     []string `json:"route_preference,omitempty"`
	Port                string   `json:"port,omitempty"`
	OS                  string   `json:"os,omitempty"`
	Password            string   `json:"password,omitempty"`
	SSHPrivateKey       string   `json:"ssh_private_key,omitempty"`
	SSHPrivateKeyPath   string   `json:"ssh_private_key_path,omitempty"`
	PreferWSLPowerShell bool     `json:"prefer_wsl_powershell,omitempty"`
	RepoCandidates      []string `json:"repo_candidates,omitempty"`
}

type subtoneLogMeta struct {
	Path    string
	Name    string
	PID     int
	ModTime time.Time
}

type launchAgentSpec struct {
	label    string
	plistRel string
}
