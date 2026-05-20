package firewall

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
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

var (
	validProtocol = map[string]bool{"tcp": true, "udp": true, "icmp": true}
	validAction   = map[string]bool{"allow": true, "deny": true, "block": true, "reject": true}
	ipRegex       = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$|^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$`)
	portRegex     = regexp.MustCompile(`^\d{1,5}$`)
	ruleIDRegex   = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
)

func validatePort(port int) bool {
	return port >= 1 && port <= 65535
}

func validateProtocol(proto string) bool {
	if proto == "" {
		return false
	}
	p := strings.ToLower(strings.TrimSpace(proto))
	return validProtocol[p]
}

func validateAction(action string) bool {
	a := strings.ToLower(strings.TrimSpace(action))
	return validAction[a]
}

func validateIP(ip string) bool {
	if ip == "" || ip == "0.0.0.0" || ip == "::" {
		return true
	}
	ip = strings.TrimSpace(ip)
	return ipRegex.MatchString(ip)
}

func validateRuleID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	return ruleIDRegex.MatchString(id) && len(id) <= 128
}

func sanitizeInput(s string) string {
	s = strings.TrimSpace(s)
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "[", "]", "<", ">", "\n", "\r", "\t", "\\"}
	for _, c := range dangerousChars {
		s = strings.ReplaceAll(s, c, "")
	}
	if len(s) > 256 {
		s = s[:256]
	}
	return s
}

func ListRules() ([]Rule, error) {
	os := runtime.GOOS
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var out []byte
	var err error
	switch os {
	case "linux":
		out, err = exec.CommandContext(ctx, "sh", "-c", "iptables -L -n --line-numbers 2>/dev/null || ufw status numbered").CombinedOutput()
	case "windows":
		out, err = exec.CommandContext(ctx, "powershell", "-Command",
			"Get-NetFirewallRule | Where-Object {$_.Direction -eq 'Inbound'} | Select-Object -First 100 Name,DisplayName,Action,Enabled,Direction | Format-List").CombinedOutput()
	}
	if err != nil {
		return nil, fmt.Errorf("firewall list failed: %w", err)
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
					rule.ID = sanitizeInput(strings.TrimSpace(parts[1]))
				}
			} else if strings.HasPrefix(line, "DisplayName") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					rule.Comment = sanitizeInput(strings.TrimSpace(parts[1]))
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
			if err == nil && validatePort(port) {
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
	if !validatePort(port) {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", port)
	}
	if !validateProtocol(protocol) {
		return fmt.Errorf("invalid protocol: %s (must be tcp/udp/icmp)", protocol)
	}
	if !validateAction(action) {
		return fmt.Errorf("invalid action: %s (must be allow/deny/block/reject)", action)
	}
	if ip != "" && !validateIP(ip) {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	os := runtime.GOOS
	portStr := strconv.Itoa(port)
	safeProto := sanitizeInput(protocol)
	safeAction := sanitizeInput(action)
	safeIP := sanitizeInput(ip)

	switch os {
	case "linux":
		args := []string{"-t", "filter", "-I", "INPUT", "-p", safeProto, "--dport", portStr}
		if safeIP != "" && safeIP != "0.0.0.0" {
			args = append(args, "-s", safeIP)
		}
		if safeAction == "allow" {
			args = append(args, "-j", "ACCEPT")
		} else {
			args = append(args, "-j", "DROP")
		}
		cmd := exec.CommandContext(ctx, "iptables", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables add rule failed: %w, output: %s", err, string(out))
		}
		return nil

	case "windows":
		ruleName := fmt.Sprintf("DevDash_%s_%d", safeProto, port)
		winAction := "Allow"
		if safeAction != "allow" && safeAction != "" {
			winAction = "Block"
		}
		cmd := exec.CommandContext(ctx, "powershell", "-Command",
			"New-NetFirewallRule", "-DisplayName", ruleName,
			"-Direction", "Inbound", "-Protocol", safeProto,
			"-LocalPort", portStr, "-Action", winAction,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("firewall rule creation failed: %w, output: %s", err, string(out))
		}
		return nil
	}
	return fmt.Errorf("unsupported OS: %s", os)
}

func RemoveRule(id string) error {
	if !validateRuleID(id) {
		return fmt.Errorf("invalid rule ID: %s", id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	os := runtime.GOOS
	safeID := sanitizeInput(id)

	switch os {
	case "linux":
		cmd := exec.CommandContext(ctx, "iptables", "-D", "INPUT", safeID)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables delete rule failed: %w, output: %s", err, string(out))
		}
		return nil

	case "windows":
		cmd := exec.CommandContext(ctx, "powershell", "-Command",
			"Remove-NetFirewallRule", "-Name", safeID)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("remove firewall rule failed: %w, output: %s", err, string(out))
		}
		return nil
	}
	return fmt.Errorf("unsupported OS: %s", os)
}

func ToggleRule(id string, enabled bool) error {
	if !validateRuleID(id) {
		return fmt.Errorf("invalid rule ID: %s", id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	os := runtime.GOOS
	safeID := sanitizeInput(id)

	switch os {
	case "windows":
		var action string
		if enabled {
			action = "Enable"
		} else {
			action = "Disable"
		}
		cmd := exec.CommandContext(ctx, "powershell", "-Command",
			action+"-NetFirewallRule", "-Name", safeID)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("toggle firewall rule failed: %w, output: %s", err, string(out))
		}
		return nil

	case "linux":
		return fmt.Errorf("Linux iptables does not support toggle by ID, use Add/Remove instead")
	}
	return fmt.Errorf("unsupported OS: %s", os)
}
