package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gxfdev/aidash/server/internal/model"
	"github.com/gxfdev/aidash/server/internal/orchestrator"
	"github.com/gxfdev/aidash/server/internal/store"
)

// Copilot is the built-in operations assistant. It reasons over live
// platform state and can execute whitelisted diagnostics inside a
// host-access sandbox container.
type Copilot struct {
	store    *store.Store
	orch     *orchestrator.Orchestrator
	gateway  *Gateway
	snapshot func() *model.HostSnapshot
}

func NewCopilot(st *store.Store, orch *orchestrator.Orchestrator, g *Gateway, snapshot func() *model.HostSnapshot) *Copilot {
	return &Copilot{store: st, orch: orch, gateway: g, snapshot: snapshot}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CopilotRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	AllowExec bool          `json:"allow_exec"`
	SandboxID string        `json:"sandbox_id"`
	MaxRounds int           `json:"max_rounds"`
}

type CopilotStep struct {
	Tool   string `json:"tool"`
	Args   string `json:"args"`
	Result string `json:"result"`
}

type CopilotResponse struct {
	Reply string        `json:"reply"`
	Model string        `json:"model"`
	Steps []CopilotStep `json:"steps"`
	Error string        `json:"error,omitempty"`
}

func toolDefs() []map[string]any {
	tool := func(name, desc string, props map[string]any, required []string) map[string]any {
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": desc,
				"parameters": map[string]any{
					"type":       "object",
					"properties": props,
					"required":   required,
				},
			},
		}
	}
	return []map[string]any{
		tool("host_overview", "获取当前服务器实时指标（CPU/内存/磁盘/网络/GPU）与容器统计", map[string]any{}, []string{}),
		tool("list_services", "列出当前部署的 AI 服务、状态、端口与宿主机权限", map[string]any{}, []string{}),
		tool("service_logs", "读取某个服务的最近日志", map[string]any{
			"service_id": map[string]any{"type": "string", "description": "服务 ID 或服务名"},
			"lines":      map[string]any{"type": "integer", "description": "读取行数，默认 80"},
		}, []string{"service_id"}),
		tool("run_command", "在具备宿主机权限的沙箱容器内执行 shell 命令（用于诊断）。仅当用户明确要求执行时调用。", map[string]any{
			"service_id": map[string]any{"type": "string", "description": "具备宿主机权限的服务 ID 或名称"},
			"command":    map[string]any{"type": "string", "description": "要执行的 shell 命令"},
		}, []string{"service_id", "command"}),
	}
}

func (c *Copilot) systemPrompt() string {
	sb := strings.Builder{}
	sb.WriteString("你是 AIDash 单机 AI 操作系统的运维助手（中文回答）。")
	sb.WriteString("你可以调用工具读取实时指标、服务状态与日志，必要时在宿主机权限沙箱容器内执行诊断命令。")
	sb.WriteString("规则：1) 不要编造数据，所有结论必须来自工具结果；2) 执行命令前先说明意图，命令需简短、只读优先；")
	sb.WriteString("3) 涉及删除、重启、改配置的操作必须提示用户手工确认；4) 回答用要点式中文，给出可执行的下一步。\n")
	if snap := c.snapshot(); snap != nil {
		sb.WriteString(fmt.Sprintf("当前主机：%s %s，CPU %.1f%%，内存 %.1f/%.1f GB，磁盘 %.1f%%。\n",
			snap.Host.Platform, snap.Host.PlatformVersion,
			snap.CPU.UsagePercent, snap.Memory.UsedGB, snap.Memory.TotalGB, snap.Disk.UsagePercent))
	}
	return sb.String()
}

// Chat runs the tool-calling loop and returns the assistant answer.
func (c *Copilot) Chat(ctx context.Context, req CopilotRequest) (*CopilotResponse, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("消息不能为空")
	}
	rounds := req.MaxRounds
	if rounds <= 0 || rounds > 6 {
		rounds = 4
	}
	messages := make([]map[string]any, 0, len(req.Messages)+4)
	messages = append(messages, map[string]any{"role": "system", "content": c.systemPrompt()})
	for _, m := range req.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		messages = append(messages, map[string]any{"role": role, "content": m.Content})
	}

	resp := &CopilotResponse{Steps: []CopilotStep{}}
	for i := 0; i < rounds; i++ {
		body := map[string]any{
			"model":       req.Model,
			"messages":    messages,
			"tools":       toolDefs(),
			"tool_choice": "auto",
			"temperature": 0.2,
			"stream":      false,
		}
		out, err := c.gateway.ChatRaw(ctx, req.Model, body)
		if err != nil {
			resp.Error = err.Error()
			return resp, nil
		}
		if m, ok := out["model"].(string); ok && m != "" {
			resp.Model = m
		}
		choice := firstChoice(out)
		if choice == nil {
			resp.Error = "模型返回了空响应"
			return resp, nil
		}
		msg, _ := choice["message"].(map[string]any)
		content, _ := msg["content"].(string)
		toolCalls, _ := msg["tool_calls"].([]any)
		if len(toolCalls) == 0 {
			resp.Reply = content
			return resp, nil
		}
		// Persist the assistant tool-call message for the next round.
		messages = append(messages, map[string]any{
			"role":       "assistant",
			"content":    content,
			"tool_calls": toolCalls,
		})
		for _, tc := range toolCalls {
			call, _ := tc.(map[string]any)
			fn, _ := call["function"].(map[string]any)
			name, _ := fn["name"].(string)
			argsRaw, _ := fn["arguments"].(string)
			args := map[string]any{}
			_ = json.Unmarshal([]byte(argsRaw), &args)
			result, execErr := c.runTool(ctx, name, args, req)
			step := CopilotStep{Tool: name, Args: argsRaw, Result: truncateStr(result, 4000)}
			if execErr != nil {
				step.Result = "错误: " + execErr.Error()
			}
			resp.Steps = append(resp.Steps, step)
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": call["id"],
				"content":      truncateStr(result, 8000),
			})
		}
		if content != "" {
			resp.Reply = content
		}
	}
	if resp.Reply == "" {
		resp.Reply = "已达到工具调用轮次上限，请根据上面的执行结果继续判断，或补充更具体的诊断方向。"
	}
	return resp, nil
}

func firstChoice(out map[string]any) map[string]any {
	choices, _ := out["choices"].([]any)
	if len(choices) == 0 {
		return nil
	}
	ch, _ := choices[0].(map[string]any)
	return ch
}

func (c *Copilot) runTool(ctx context.Context, name string, args map[string]any, req CopilotRequest) (string, error) {
	switch name {
	case "host_overview":
		return c.hostOverview(ctx), nil
	case "list_services":
		return c.listServices()
	case "service_logs":
		return c.serviceLogs(ctx, str(args["service_id"]), int(num(args["lines"])))
	case "run_command":
		return c.runCommand(ctx, str(args["service_id"]), str(args["command"]), req)
	default:
		return "", fmt.Errorf("未知工具: %s", name)
	}
}

func (c *Copilot) hostOverview(ctx context.Context) string {
	snap := c.snapshot()
	engine := c.orch.EngineSnapshot(ctx)
	payload := map[string]any{
		"host":       snap.Host,
		"cpu":        snap.CPU,
		"memory":     snap.Memory,
		"disk":       snap.Disk,
		"network":    snap.Network,
		"load":       snap.Load,
		"gpu":        snap.GPU,
		"containers": snap.Containers,
		"docker":     engine,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return string(b)
}

func (c *Copilot) listServices() (string, error) {
	services, err := c.store.ListServices()
	if err != nil {
		return "", err
	}
	type row struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Kind       string   `json:"kind"`
		Status     string   `json:"status"`
		Health     string   `json:"health"`
		Image      string   `json:"image"`
		Model      string   `json:"model,omitempty"`
		Ports      []string `json:"endpoints"`
		HostAccess bool     `json:"host_access"`
	}
	rows := make([]row, 0, len(services))
	for _, svc := range services {
		rows = append(rows, row{
			ID: svc.ID, Name: svc.Name, Kind: svc.Kind, Status: svc.Status,
			Health: svc.Health, Image: svc.Image, Model: svc.ModelName,
			Ports: svc.Endpoints, HostAccess: svc.Spec.HostAccess.Enabled,
		})
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	return string(b), nil
}

func (c *Copilot) findService(idOrName string) (*model.Service, error) {
	if idOrName == "" {
		return nil, fmt.Errorf("缺少服务 ID")
	}
	if svc, err := c.store.GetService(idOrName); err == nil {
		return svc, nil
	}
	return c.store.GetServiceByName(idOrName)
}

func (c *Copilot) serviceLogs(ctx context.Context, id string, lines int) (string, error) {
	svc, err := c.findService(id)
	if err != nil {
		return "", fmt.Errorf("找不到服务 %s", id)
	}
	if lines <= 0 || lines > 200 {
		lines = 80
	}
	logs, err := c.orch.Logs(ctx, svc.ID, lines)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(logs) == "" {
		return "（暂无日志输出）", nil
	}
	return truncateStr(logs, 8000), nil
}

func (c *Copilot) runCommand(ctx context.Context, id, command string, req CopilotRequest) (string, error) {
	if !req.AllowExec {
		return "", fmt.Errorf("用户未开启「允许执行命令」，请提示用户在对话框中打开该开关")
	}
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("命令为空")
	}
	svc, err := c.findService(id)
	if err != nil {
		return "", fmt.Errorf("找不到服务 %s", id)
	}
	if !svc.Spec.HostAccess.Enabled {
		return "", fmt.Errorf("服务 %s 未开启宿主机权限，不能执行命令", svc.Name)
	}
	if svc.Status != orchestrator.StatusRunning {
		return "", fmt.Errorf("服务 %s 当前状态为 %s，无法执行命令", svc.Name, svc.Status)
	}
	output, err := c.orch.RunCommand(ctx, svc.ID, command, 60*time.Second)
	if err != nil && output == "" {
		return "", err
	}
	if err != nil {
		return output + "\n(命令返回错误: " + err.Error() + ")", nil
	}
	return truncateStr(output, 8000), nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…（已截断）"
}
