package src_v3

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type managedRoleSummary struct {
	Role        string
	BrowserPIDs []int
	DaemonPIDs  []int
	IsWindows   bool
	IsHeadless  bool
}

func killDialtoneResourcesLocal() error {
	resources, err := listResourcesLocal(true)
	if err != nil {
		return err
	}
	var firstErr error
	for _, resource := range resources {
		if err := killResourceLocal(resource.PID, resource.IsWindows); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func listResourcesLocal(includeChrome bool) ([]Resource, error) {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", resourceListingScriptWindows(includeChrome)).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("list chrome resources failed: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return parseResourceLines(string(out)), nil
	default:
		out, err := exec.Command("bash", "-lc", resourceListingScriptPOSIX(includeChrome)).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("list chrome resources failed: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return parseResourceLines(string(out)), nil
	}
}

func listResourcesByTarget(host string, includeChrome bool) ([]Resource, error) {
	host = effectiveChromeTargetHost(host)
	if isLocalHost(host) {
		return listResourcesLocal(includeChrome)
	}
	node, err := resolveMeshNode(host)
	if err != nil {
		return nil, err
	}
	return listRemoteResources(node, includeChrome)
}

func listRemoteResources(node sshv1.MeshNode, includeChrome bool) ([]Resource, error) {
	var (
		out string
		err error
	)
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		out, err = sshv1.RunNodeCommand(node.Name, resourceListingScriptWindows(includeChrome), sshv1.CommandOptions{})
	} else {
		out, err = sshv1.RunNodeCommand(node.Name, resourceListingScriptPOSIX(includeChrome), sshv1.CommandOptions{})
	}
	if err != nil {
		return nil, err
	}
	return parseResourceLines(out), nil
}

func killResourceLocal(pid int, isWindows bool) error {
	if pid <= 0 {
		return nil
	}
	if isWindows || runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	return exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}

func killResourceByTarget(host string, pid int, isWindows bool) error {
	host = effectiveChromeTargetHost(host)
	if isLocalHost(host) {
		return killResourceLocal(pid, isWindows)
	}
	node, err := resolveMeshNode(host)
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") || isWindows {
		_, err = sshv1.RunNodeCommand(node.Name, fmt.Sprintf("Stop-Process -Id %d -Force -ErrorAction Stop", pid), sshv1.CommandOptions{})
		return err
	}
	_, err = sshv1.RunNodeCommand(node.Name, fmt.Sprintf("kill -9 %d", pid), sshv1.CommandOptions{})
	return err
}

func summarizeManagedResources(resources []Resource, roleFilter string) []managedRoleSummary {
	roleFilter = strings.TrimSpace(roleFilter)
	if roleFilter != "" {
		roleFilter = normalizeRole(roleFilter)
	}
	byRole := map[string]*managedRoleSummary{}
	for _, resource := range resources {
		role := normalizeRole(resource.Role)
		if roleFilter != "" && role != roleFilter {
			continue
		}
		item := byRole[role]
		if item == nil {
			item = &managedRoleSummary{Role: role}
			byRole[role] = item
		}
		item.IsWindows = item.IsWindows || resource.IsWindows
		if strings.EqualFold(strings.TrimSpace(resource.Origin), "chrome") {
			item.BrowserPIDs = append(item.BrowserPIDs, resource.PID)
			item.IsHeadless = item.IsHeadless || resource.IsHeadless
			continue
		}
		item.DaemonPIDs = append(item.DaemonPIDs, resource.PID)
	}
	roles := make([]string, 0, len(byRole))
	for role := range byRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	out := make([]managedRoleSummary, 0, len(roles))
	for _, role := range roles {
		item := byRole[role]
		sort.Ints(item.BrowserPIDs)
		sort.Ints(item.DaemonPIDs)
		out = append(out, *item)
	}
	return out
}

func formatPIDList(pids []int) string {
	if len(pids) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		parts = append(parts, strconv.Itoa(pid))
	}
	return strings.Join(parts, ",")
}

func resourceListingScriptWindows(includeChrome bool) string {
	var b strings.Builder
	b.WriteString("Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'dialtone_chrome_v3.exe' } | ForEach-Object { ")
	b.WriteString("$m=[regex]::Match($_.CommandLine,'--role(?:=|\\s+\"?)([^\" ]+)'); ")
	b.WriteString("if(-not $m.Success){ $m=[regex]::Match($_.CommandLine,'\"--role\"\\s+\"([^\"]+)\"') }; ")
	b.WriteString("if($m.Success -and $m.Groups[1].Value){ Write-Output ($_.ProcessId.ToString() + \"`t\" + $m.Groups[1].Value + \"`tdaemon`t1`t0\") } ")
	b.WriteString("}; ")
	if includeChrome {
		b.WriteString("Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and $_.CommandLine -notlike '*--type=*' -and $_.CommandLine -match '--dialtone-role=' } | ForEach-Object { ")
		b.WriteString("$m=[regex]::Match($_.CommandLine,'--dialtone-role=([^\" ]+)'); ")
		b.WriteString("if($m.Success -and $m.Groups[1].Value){ $headless='0'; if($_.CommandLine -like '*--headless*'){ $headless='1' }; Write-Output ($_.ProcessId.ToString() + \"`t\" + $m.Groups[1].Value + \"`tchrome`t1`t\" + $headless) } ")
		b.WriteString("}; ")
	}
	return b.String()
}

func resourceListingScriptPOSIX(includeChrome bool) string {
	includeChromeInt := 0
	if includeChrome {
		includeChromeInt = 1
	}
	return fmt.Sprintf(`includeChrome=%d
ps -eo pid=,args= | while IFS= read -r line; do
  [ -z "$line" ] && continue
  set -- $line
  [ $# -lt 1 ] && continue
  pid=$1
  shift
  rest="$*"
  case "$rest" in
    *dialtone_chrome_v3* )
      role=$(printf '%%s\n' "$rest" | sed -nE 's/.*--role(=|[[:space:]]+)"?([^" ]+).*/\2/p')
      if [ -n "$role" ]; then
        printf '%%s\t%%s\tdaemon\t0\t0\n' "$pid" "$role"
      fi
    ;;
  esac
  if [ "$includeChrome" -eq 1 ]; then
    case "$rest" in
      *--dialtone-role=* )
        case "$rest" in
          *--type=* ) ;;
          *chrome*|*chromium* )
            role=$(printf '%%s\n' "$rest" | sed -nE 's/.*--dialtone-role=([^" ]+).*/\1/p')
            if [ -n "$role" ]; then
              headless=0
              case "$rest" in
                *--headless* ) headless=1 ;;
              esac
              printf '%%s\t%%s\tchrome\t0\t%%s\n' "$pid" "$role" "$headless"
            fi
          ;;
        esac
      ;;
    esac
  fi
done`, includeChromeInt)
}

func parseResourceLines(raw string) []Resource {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]Resource, 0, len(lines))
	seen := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil || pid <= 0 {
			continue
		}
		role := normalizeRole(strings.TrimSpace(fields[1]))
		origin := strings.TrimSpace(fields[2])
		resource := Resource{
			PID:        pid,
			Role:       role,
			Origin:     origin,
			IsWindows:  parseTabBool(fields[3]),
			IsHeadless: parseTabBool(fields[4]),
		}
		key := fmt.Sprintf("%s:%s:%d", resource.Origin, resource.Role, resource.PID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, resource)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		if out[i].Origin != out[j].Origin {
			return out[i].Origin < out[j].Origin
		}
		return out[i].PID < out[j].PID
	})
	return out
}

func parseTabBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func closeManagedBrowsersByTarget(host, role string) ([]managedRoleSummary, []managedRoleSummary, error) {
	host = effectiveChromeTargetHost(host)
	beforeResources, err := listResourcesByTarget(host, true)
	if err != nil {
		return nil, nil, err
	}
	before := summarizeManagedResources(beforeResources, role)
	if len(before) == 0 {
		return before, before, nil
	}
	for _, item := range before {
		if len(item.BrowserPIDs) == 0 {
			continue
		}
		_, err := sendCommandByTarget(host, commandRequest{
			Command: "close",
			Role:    item.Role,
		}, false)
		if err != nil {
			continue
		}
	}
	time.Sleep(500 * time.Millisecond)
	afterResources, err := listResourcesByTarget(host, true)
	if err != nil {
		return before, nil, err
	}
	after := summarizeManagedResources(afterResources, role)
	afterByRole := map[string]managedRoleSummary{}
	for _, item := range after {
		afterByRole[item.Role] = item
	}
	for _, item := range before {
		current := afterByRole[item.Role]
		if len(current.BrowserPIDs) == 0 {
			continue
		}
		for _, pid := range current.BrowserPIDs {
			if err := killResourceByTarget(host, pid, current.IsWindows); err != nil {
				return before, after, err
			}
		}
	}
	time.Sleep(250 * time.Millisecond)
	finalResources, err := listResourcesByTarget(host, true)
	if err != nil {
		return before, nil, err
	}
	return before, summarizeManagedResources(finalResources, role), nil
}
