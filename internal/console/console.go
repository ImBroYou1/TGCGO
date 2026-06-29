package console

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func RunBash(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))

	if ctx.Err() == context.DeadlineExceeded {
		return "❌ Command timed out (15s limit)"
	}

	if err != nil {
		if result != "" {
			return result
		}
		return fmt.Sprintf("❌ %s", err.Error())
	}
	if result == "" {
		return "✅ OK"
	}
	if len(result) > 3500 {
		result = result[:3500] + "\n... (truncated)"
	}
	return result
}

func GetQuickCommands() []struct{ Name, Cmd string } {
	return []struct{ Name, Cmd string }{
		{"📋 Uptime", "uptime"},
		{"📊 Top CPU", "ps aux --sort=-%cpu | head -8"},
		{"💾 Disks", "df -h | grep -E '^/dev|Filesystem'"},
		{"🧠 RAM", "free -h"},
		{"🌐 Network", "ip -br a"},
		{"📝 Logs", "journalctl -n 15 --no-pager 2>&1"},
		{"👥 Who", "who"},
	}
}
