package firewall

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Rule struct {
	ID       string `json:"id"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"`
	IP       string `json:"ip"`
	Comment  string `json:"comment"`
	Enabled  bool   `json:"enabled"`
	Proto    string `json:"proto"`
	SrcIP    string `json:"src_ip"`
	Note     string `json:"note"`
}

func ListRules() ([]Rule, error) {
	os := runtime.GOOS
	var out []byte
	var err error
	switch os {
	case "linux":
		out, err = exec.Command("sh", "-c", "iptables -L -n --line-numbers 2>/dev/null || ufw status numbered").CombinedOutput()
	case "windows":
		out, err = exec.Command("powershell", "-Command",
			"Get-NetFirewallRule | Where-Object {$_.Direction -eq 'Inbound'} | Select-Object -First 100 Name,DisplayName,Action,Enabled,Direction | Format-List").CombinedOutput()
	}
	if err != nil {
		return nil, err
	}
	return parseRules(string(out), os), nil
}

func parseRules(output, os string) []Rule {
	switch os {
	case "windows":
		return parseWindowsRules(output)
	case "linux":
		return parseLinuxRules(output)
	}
	return []Rule{}
}

func parseWindowsRules(output string) []Rule {
	var rules []Rule
	sections := strings.Split(output, "\r\n\r\n")
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		rule := Rule{}
		lines := strings.Split(section, "\r\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					rule.ID = strings.TrimSpace(parts[1])
				}
			} else if strings.HasPrefix(line, "DisplayName") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					rule.Comment = strings.TrimSpace(parts[1])
					rule.Note = rule.Comment
				}
			} else if strings.HasPrefix(line, "Action") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					action := strings.TrimSpace(parts[1])
					if action == "Allow" {
						rule.Action = "allow"
					} else {
						rule.Action = "block"
					}
				}
			} else if strings.HasPrefix(line, "Enabled") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					rule.Enabled = strings.TrimSpace(parts[1]) == "True"
				}
			}
		}
		if rule.ID != "" {
			rule.Protocol = "tcp"
			rule.Proto = "tcp"
			rules = append(rules, rule)
		}
	}
	return rules
}

func parseLinuxRules(output string) []Rule {
	var rules []Rule
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Chain") || strings.HasPrefix(line, "num") || strings.HasPrefix(line, "Status") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		rule := Rule{
			ID:       fields[0],
			Protocol: "tcp",
			Proto:    "tcp",
			Enabled:  true,
		}
		if len(fields) > 1 {
			rule.Action = strings.ToLower(fields[1])
		}
		if len(fields) > 2 {
			rule.Protocol = fields[2]
			rule.Proto = rule.Protocol
		}
		if len(fields) > 3 {
			port, err := strconv.Atoi(fields[3])
			if err == nil {
				rule.Port = port
			}
		}
		if len(fields) > 4 {
			rule.IP = fields[4]
			rule.SrcIP = rule.IP
		}
		rules = append(rules, rule)
	}
	return rules
}

func AddRule(port int, protocol, action, ip string) error {
	os := runtime.GOOS
	portStr := strconv.Itoa(port)
	switch os {
	case "linux":
		rule := "-A INPUT -p " + protocol + " --dport " + portStr
		if ip != "" {
			rule += " -s " + ip
		}
		if action == "allow" {
			rule += " -j ACCEPT"
		} else {
			rule += " -j DROP"
		}
		return exec.Command("sh", "-c", "iptables "+rule).Run()
	case "windows":
		ruleName := "DevDash_" + protocol + "_" + portStr
		winAction := "Allow"
		if action == "block" {
			winAction = "Block"
		}
		return exec.Command("powershell", "-Command",
			"New-NetFirewallRule -DisplayName '"+ruleName+"' -Direction Inbound -Protocol "+protocol+" -LocalPort "+portStr+" -Action "+winAction).Run()
	}
	return nil
}

func RemoveRule(id string) error {
	os := runtime.GOOS
	switch os {
	case "linux":
		return exec.Command("sh", "-c", "iptables -D INPUT "+id).Run()
	case "windows":
		return exec.Command("powershell", "-Command", "Remove-NetFirewallRule -Name "+id).Run()
	}
	return nil
}

func ToggleRule(id string, enabled bool) error {
	os := runtime.GOOS
	switch os {
	case "windows":
		if enabled {
			return exec.Command("powershell", "-Command", "Enable-NetFirewallRule -Name "+id).Run()
		}
		return exec.Command("powershell", "-Command", "Disable-NetFirewallRule -Name "+id).Run()
	case "linux":
		if enabled {
			return exec.Command("sh", "-c", "iptables -A INPUT "+id).Run()
		}
		return exec.Command("sh", "-c", "iptables -D INPUT "+id).Run()
	}
	return nil
}
