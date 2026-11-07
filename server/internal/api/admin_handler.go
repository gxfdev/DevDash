package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listUsers(c *gin.Context) {
	users, err := h.store.ListUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"users": users})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *Handler) createUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 8 {
		fail(c, http.StatusBadRequest, "用户名必填，密码至少 8 位")
		return
	}
	switch req.Role {
	case "admin", "operator", "viewer":
	default:
		req.Role = "viewer"
	}
	user, err := h.store.CreateUser(req.Username, req.Password, req.Role, true)
	if err != nil {
		fail(c, http.StatusBadRequest, "创建失败（用户名可能已存在）")
		return
	}
	h.audit(c, "创建用户", req.Username, req.Role, "ok")
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

type updateUserRequest struct {
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (h *Handler) updateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的用户 ID")
		return
	}
	user, err := h.store.GetUser(id)
	if err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if req.Role != "" {
		switch req.Role {
		case "admin", "operator", "viewer":
			if err := h.store.UpdateUserRole(id, req.Role); err != nil {
				fail(c, http.StatusInternalServerError, err.Error())
				return
			}
		default:
			fail(c, http.StatusBadRequest, "无效的角色")
			return
		}
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			fail(c, http.StatusBadRequest, "密码至少 8 位")
			return
		}
		if err := h.store.UpdateUserPassword(id, req.Password, false); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	h.audit(c, "更新用户", user.Username, req.Role, "ok")
	updated, _ := h.store.GetUser(id)
	ok(c, gin.H{"user": updated})
}

func (h *Handler) deleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的用户 ID")
		return
	}
	uid, _ := c.Get("user_id")
	if self, _ := uid.(int64); self == id {
		fail(c, http.StatusBadRequest, "不能删除当前登录用户")
		return
	}
	user, err := h.store.GetUser(id)
	if err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if err := h.store.DeleteUser(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "删除用户", user.Username, "", "ok")
	ok(c, gin.H{"message": "用户已删除"})
}

func (h *Handler) listAudit(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	logs, err := h.store.ListAudit(c.Query("q"), limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"logs": logs})
}

var settingDefaults = map[string]string{
	"site_name":              "AIDash 单机 AI 操作系统",
	"copilot_model":          "",
	"default_model":          "",
	"metrics_retention_days": "30",
	"gateway_enabled":        "true",
	"deploy_host_note":       "",
}

func (h *Handler) getSettings(c *gin.Context) {
	out := map[string]string{}
	for k, def := range settingDefaults {
		out[k] = h.store.GetSetting(k, def)
	}
	ok(c, gin.H{"settings": out, "defaults": settingDefaults})
}

func (h *Handler) updateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	changed := []string{}
	for k, v := range req {
		if _, allowed := settingDefaults[k]; !allowed {
			continue
		}
		if err := h.store.SetSetting(k, v); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		changed = append(changed, k)
	}
	h.audit(c, "更新系统设置", strings.Join(changed, ","), "", "ok")
	h.getSettings(c)
}

var _ = time.Now
