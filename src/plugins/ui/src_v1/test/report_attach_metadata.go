package test

import (
	"fmt"
	"os"
	"strings"
)

const (
	attachMetadataStart = "<!-- ATTACH_METADATA_START -->"
	attachMetadataEnd   = "<!-- ATTACH_METADATA_END -->"
)

func WriteAttachMetadataReport(reportPath, attachNode string, before, after *RemoteBrowserInventory) error {
	reportPath = strings.TrimSpace(reportPath)
	if reportPath == "" {
		return nil
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	content := string(raw)
	content = stripAttachMetadataBlock(content)

	hostNode := strings.TrimSpace(attachNode)
	if before != nil && strings.TrimSpace(before.Node) != "" {
		hostNode = strings.TrimSpace(before.Node)
	}
	beforeCount := inventoryCount(before)
	afterCount := inventoryCount(after)

	var block strings.Builder
	block.WriteString(attachMetadataStart)
	block.WriteString("\n")
	block.WriteString("```yaml\n")
	block.WriteString(fmt.Sprintf("chrome_hostnode: %s\n", yamlValue(hostNode)))
	block.WriteString(fmt.Sprintf("chrome_count_before: %s\n", yamlValue(beforeCount)))
	block.WriteString(fmt.Sprintf("chrome_count_after: %s\n", yamlValue(afterCount)))
	block.WriteString("```\n")
	block.WriteString(attachMetadataEnd)
	block.WriteString("\n\n")

	lines := strings.Split(content, "\n")
	insertAt := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			insertAt = i + 1
			break
		}
	}
	updated := append([]string{}, lines[:insertAt]...)
	updated = append(updated, "", block.String())
	updated = append(updated, lines[insertAt:]...)
	out := strings.Join(updated, "\n")
	return os.WriteFile(reportPath, []byte(out), 0644)
}

func stripAttachMetadataBlock(content string) string {
	start := strings.Index(content, attachMetadataStart)
	end := strings.Index(content, attachMetadataEnd)
	if start < 0 || end < 0 || end < start {
		return content
	}
	end += len(attachMetadataEnd)
	for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
		end++
	}
	return content[:start] + content[end:]
}

func inventoryCount(inv *RemoteBrowserInventory) string {
	if inv == nil || inv.ChromeCount < 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", inv.ChromeCount)
}

func yamlValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return v
}
