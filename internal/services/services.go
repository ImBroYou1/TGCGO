package services

import (
	"TGCGO/config"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getPath() string {
	return "data/custom_services.json"
}

func Load() []string {
	data, err := os.ReadFile(getPath())
	if err != nil {
		return []string{"sshd", "smb", "nmb", "nginx", "docker", "cron"}
	}

	var result struct {
		Services []string `json:"services"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return []string{"sshd", "smb", "nmb", "nginx", "docker", "cron"}
	}

	return result.Services
}

func Save(svcs []string) {
	os.MkdirAll("data", 0755)

	result := struct {
		Services []string `json:"services"`
	}{Services: svcs}

	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(getPath(), data, 0644)
}

func AddService(name string) {
	svcs := Load()
	svcs = append(svcs, name)
	Save(svcs)
}

func Remove(name string) {
	svcs := Load()
	var newSvcs []string
	for _, s := range svcs {
		if s != name {
			newSvcs = append(newSvcs, s)
		}
	}
	Save(newSvcs)
}

func isValidServiceName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

func GetInfo() string {
	svcs := Load()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚙️ *%s*\n\n", config.T("services")))

	for _, s := range svcs {
		s = strings.TrimSpace(s)
		if !isValidServiceName(s) {
			continue
		}
		cmd := exec.Command("systemctl", "is-active", s)
		out, _ := cmd.Output()
		status := strings.TrimSpace(string(out))

		icon := "⚪"
		state := config.T("not_found")
		switch status {
		case "active":
			icon = "🟢"
			state = config.T("running")
		case "inactive":
			icon = "🔴"
			state = config.T("stopped")
		}

		sb.WriteString(fmt.Sprintf("%s `%s` — %s\n", icon, s, state))
	}

	return sb.String()
}

func Manage(service, action string) string {
	service = strings.TrimSpace(service)
	if !isValidServiceName(service) {
		return "❌ Invalid service name"
	}
	if action != "start" && action != "stop" && action != "restart" {
		return "❌ Invalid action"
	}

	desc := config.T(action)

	cmd := exec.Command("sudo", "systemctl", action, service)
	out, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "✅ OK"
	}

	return fmt.Sprintf("%s `%s`\n```\n%s\n```", desc, service, result)
}
