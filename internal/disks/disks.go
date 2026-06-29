package disks

import (
	"TGCGO/config"
	"fmt"
	"os/exec"
	"strings"
)

type MountPoint struct {
	Device string
	Mount  string
}

func GetInfo() string {
	cmd := exec.Command("bash", "-c", "df -h -t ext4 -t ntfs -t xfs -t btrfs -t fuseblk 2>/dev/null | tail -n +2")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return fmt.Sprintf("%s\n\n%s", config.T("disks_title"), config.T("no_mounts"))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n\n", config.T("disks_title")))

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 6 {
			device := parts[0]
			size := parts[1]
			used := parts[2]
			mount := parts[5]
			percent := parts[4]

			pct := 0
			fmt.Sscanf(percent, "%d%%", &pct)
			bar := strings.Repeat("█", pct/10) + strings.Repeat("░", 10-pct/10)

			sb.WriteString(fmt.Sprintf("📀 `%s`\n├ `%s`: %s/%s\n└ [%s] %d%%\n\n", device, mount, used, size, bar, pct))
		}
	}

	return sb.String()
}

func GetMounted() []MountPoint {
	cmd := exec.Command("bash", "-c", "mount | grep '^/dev' | awk '{print $1,$3}'")
	out, _ := cmd.Output()

	var mounts []MountPoint
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			mounts = append(mounts, MountPoint{Device: parts[0], Mount: parts[1]})
		}
	}

	return mounts
}

func Mount(device, path, options string) string {
	cmdMkdir := exec.Command("sudo", "mkdir", "-p", path)
	if out, err := cmdMkdir.CombinedOutput(); err != nil {
		return fmt.Sprintf("Error creating mount point: %s\n%s", err.Error(), strings.TrimSpace(string(out)))
	}
	cmdMount := exec.Command("sudo", "mount", "-o", options, device, path)
	out, _ := cmdMount.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "✅ Mounted successfully"
	}
	return result
}

func Umount(device string) string {
	cmd := exec.Command("sudo", "umount", "-l", device)
	out, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "✅ Unmounted successfully"
	}
	return result
}
