package system

import (
	"TGCGO/config"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

func GetInfo() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n\n", config.T("system_title")))

	cpuInfo, _ := cpu.Info()
	cpuPercent, _ := cpu.Percent(0, false)
	cores := runtime.NumCPU()

	sb.WriteString(fmt.Sprintf("🔥 *%s*\n", config.T("cpu")))
	if len(cpuInfo) > 0 {
		model := strings.TrimSpace(cpuInfo[0].ModelName)
		model = strings.Replace(model, "(R)", "", -1)
		model = strings.Replace(model, "(TM)", "", -1)
		sb.WriteString(fmt.Sprintf("├ %s: `%s`\n", config.T("model"), model))
		sb.WriteString(fmt.Sprintf("├ %s: `%d`\n", config.T("cores"), cores))
		if cpuInfo[0].Mhz > 0 {
			sb.WriteString(fmt.Sprintf("├ %s: `%.0f MHz`\n", config.T("frequency"), cpuInfo[0].Mhz))
		}
	}
	var cpuUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}
	cpuBar := strings.Repeat("█", int(cpuUsage/10)) + strings.Repeat("░", 10-int(cpuUsage/10))
	sb.WriteString(fmt.Sprintf("├ %s: [%s] %.1f%%\n", config.T("usage"), cpuBar, cpuUsage))

	// Temperature
	temps, err := host.SensorsTemperatures()
	tempStr := "N/A"
	if err == nil {
		for _, t := range temps {
			sensorLower := strings.ToLower(t.SensorKey)
			if strings.HasPrefix(sensorLower, "cpu") || strings.Contains(sensorLower, "temp") || strings.Contains(sensorLower, "core") {
				tempStr = fmt.Sprintf("%.1f°C", t.Temperature)
				break
			}
		}
	}
	sb.WriteString(fmt.Sprintf("└ %s: `%s`\n\n", config.T("temperature"), tempStr))

	vmem, _ := mem.VirtualMemory()
	ramPercent := vmem.UsedPercent
	ramBar := strings.Repeat("█", int(ramPercent/10)) + strings.Repeat("░", 10-int(ramPercent/10))

	sb.WriteString(fmt.Sprintf("🧠 *%s*\n", config.T("ram")))
	sb.WriteString(fmt.Sprintf("├ [%s] %.1f%%\n", ramBar, ramPercent))
	sb.WriteString(fmt.Sprintf("├ %s: `%.1f` / `%.1f` GB\n",
		config.T("used"),
		float64(vmem.Used)/1024/1024/1024,
		float64(vmem.Total)/1024/1024/1024))
	sb.WriteString(fmt.Sprintf("└ %s: `%.1f` GB\n\n", config.T("available"), float64(vmem.Available)/1024/1024/1024))

	uptime, _ := host.Uptime()
	days := uptime / 86400
	hours := (uptime % 86400) / 3600
	minutes := (uptime % 3600) / 60

	uptimeVal := fmt.Sprintf(config.T("uptime_val"), days, hours, minutes)
	loadAvg, _ := load.Avg()

	sb.WriteString(fmt.Sprintf("📊 *%s*\n", config.T("uptime")))
	sb.WriteString(fmt.Sprintf("├ %s: `%s`\n", config.T("uptime"), uptimeVal))
	sb.WriteString(fmt.Sprintf("└ %s: `%.1f %.1f %.1f`\n", config.T("load"), loadAvg.Load1, loadAvg.Load5, loadAvg.Load15))

	return sb.String()
}

func RunBash(command string) string {
	out, err := exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("❌ %s\n%s", err.Error(), string(out))
	}
	return strings.TrimSpace(string(out))
}

func Run(cmd string) string {
	return RunBash(cmd)
}
