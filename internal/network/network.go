package network

import (
	"TGCGO/config"
	"fmt"
	"os/exec"
	"strings"
)

func GetInfo() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n\n", config.T("network_title")))

	localIP := getLocalIP()
	publicIP := getPublicIP()

	sb.WriteString(fmt.Sprintf("📡 *%s:* `%s`\n", config.T("local_ip"), localIP))
	sb.WriteString(fmt.Sprintf("🌍 *%s:* `%s`\n\n", config.T("public_ip"), publicIP))

	sb.WriteString(fmt.Sprintf("👤 *%s:*\n", config.T("ssh_sessions")))
	sshSessions := getSSHSessions()
	if len(sshSessions) == 0 {
		sb.WriteString(fmt.Sprintf("└ ✅ %s\n", config.T("no_active")))
	} else {
		for _, s := range sshSessions {
			sb.WriteString(fmt.Sprintf("├ `%s`\n", s))
		}
	}

	return sb.String()
}

func getLocalIP() string {
	cmd := exec.Command("bash", "-c", "ip -4 route get 1.1.1.1 | awk '{print $7}' | head -1")
	out, _ := cmd.Output()
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		cmd = exec.Command("bash", "-c", "hostname -I | awk '{print $1}'")
		out, _ = cmd.Output()
		ip = strings.TrimSpace(string(out))
	}
	if ip == "" {
		ip = "N/A"
	}
	return ip
}

func getPublicIP() string {
	cmd := exec.Command("bash", "-c", "curl -s --max-time 3 ifconfig.me 2>/dev/null")
	out, _ := cmd.Output()
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		ip = "N/A"
	}
	return ip
}

func getSSHSessions() []string {
	cmd := exec.Command("bash", "-c", "ss -tnp 2>/dev/null | grep ':22 ' | grep ESTAB | awk '{print $5}' | cut -d':' -f1")
	out, _ := cmd.Output()
	ips := strings.Split(strings.TrimSpace(string(out)), "\n")

	var result []string
	seen := make(map[string]bool)
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" && ip != "0.0.0.0" && ip != "*" && !seen[ip] {
			seen[ip] = true
			result = append(result, ip)
		}
	}
	return result
}

func CheckPort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return "❌ Port is empty"
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return "❌ Invalid port format"
		}
	}
	cmd := exec.Command("ss", "-tlnp")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, ":"+port+" ") {
				return fmt.Sprintf(config.T("port_open"), port)
			}
		}
	}
	return fmt.Sprintf(config.T("port_closed"), port)
}

func GetConnections() string {
	cmd := exec.Command("bash", "-c", "ss -tunap 2>/dev/null | head -25")
	out, _ := cmd.Output()
	return fmt.Sprintf("🔌 *%s*\n\n```\n%s\n```", config.T("active_connections"), strings.TrimSpace(string(out)))
}

func GetListeningPorts() string {
	cmd := exec.Command("bash", "-c", "ss -tlnp 2>/dev/null | head -25")
	out, _ := cmd.Output()
	return fmt.Sprintf("📡 *%s*\n\n```\n%s\n```", config.T("listening_ports"), strings.TrimSpace(string(out)))
}
