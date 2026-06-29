package processes

import (
	"TGCGO/config"
	"fmt"
	"os/exec"
	"strings"
)

func GetTopProcesses() string {
	cmd := exec.Command("bash", "-c", "ps aux --sort=-%cpu | head -15")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return config.T("no_commands")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 *%s*\n\n", config.T("top_processes")))

	for i, line := range lines {
		if i == 0 {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 11 {
			pid := parts[1]
			cpu := parts[2]
			mem := parts[3]
			command := strings.Join(parts[10:], " ")
			if len(command) > 30 {
				command = command[:30] + "..."
			}
			sb.WriteString(fmt.Sprintf("├ PID:`%s` CPU:`%s%%` MEM:`%s%%`\n│ `%s`\n\n", pid, cpu, mem, command))
		}
	}

	return sb.String()
}

func KillProcess(pid string) string {
	pid = strings.TrimSpace(pid)
	if pid == "" {
		return "❌ PID is empty"
	}
	for _, c := range pid {
		if c < '0' || c > '9' {
			return "❌ Invalid PID format"
		}
	}
	cmd := exec.Command("sudo", "kill", "-9", pid)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		if result != "" {
			return fmt.Sprintf("❌ %s", result)
		}
		return fmt.Sprintf("❌ %s", err.Error())
	}
	return fmt.Sprintf("✅ PID %s killed", pid)
}

func GetCount() int {
	cmd := exec.Command("bash", "-c", "ps aux | wc -l")
	out, _ := cmd.Output()
	count := 0
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count - 1
}
