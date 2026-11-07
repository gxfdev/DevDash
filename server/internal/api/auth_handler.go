package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/model"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	user, err := h.store.GetUserByName(req.Username)
	if err != nil || !h.store.CheckPassword(user, req.Password) {
		h.audit(c, "登录失败", req.Username, "用户名或密码错误", "failed")
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	token, err := h.auth.Sign(user, 24*time.Hour)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.store.TouchLogin(user.ID)
	h.audit(c, "登录", user.Username, "登录成功", "ok")
	ok(c, gin.H{"token": token, "user": user})
}

func (h *Handler) me(c *gin.Context) {
	id, _ := c.Get("user_id")
	uid, _ := id.(int64)
	user, err := h.store.GetUser(uid)
	if err != nil {
		fail(c, http.StatusUnauthorized, "用户不存在")
		return
	}
	ok(c, gin.H{"user": user})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	if len(req.NewPassword) < 8 {
		fail(c, http.StatusBadRequest, "新密码至少 8 位")
		return
	}
	id, _ := c.Get("user_id")
	uid, _ := id.(int64)
	user, err := h.store.GetUser(uid)
	if err != nil {
		fail(c, http.StatusUnauthorized, "用户不存在")
		return
	}
	if !h.store.CheckPassword(user, req.OldPassword) {
		fail(c, http.StatusBadRequest, "原密码不正确")
		return
	}
	if err := h.store.UpdateUserPassword(uid, req.NewPassword, false); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "修改密码", user.Username, "", "ok")
	ok(c, gin.H{"message": "密码已更新"})
}

func (h *Handler) audit(c *gin.Context, action, target, detail, result string) {
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	uname, _ := username.(string)
	r, _ := role.(string)
	if uname == "" {
		uname = target
	}
	_ = h.store.InsertAudit(&model.AuditLog{
		User: uname, Role: r, Action: action, Target: target,
		Detail: detail, IP: c.ClientIP(), Result: result,
		CreatedAt: time.Now(),
	})
}
