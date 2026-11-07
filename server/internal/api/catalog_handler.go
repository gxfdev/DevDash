package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) catalogList(c *gin.Context) {
	ok(c, gin.H{
		"models":           h.cat.ModelsOnly(),
		"apps":             h.cat.AppsOnly(),
		"model_categories": h.cat.Categories("model"),
		"app_categories":   h.cat.Categories("app"),
	})
}

func (h *Handler) catalogItem(c *gin.Context) {
	item, found := h.cat.Get(c.Param("id"))
	if !found {
		fail(c, http.StatusNotFound, "目录中不存在该条目")
		return
	}
	ok(c, item)
}
