package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gxfdev/aidash/server/internal/modelhub"
)

func (h *Handler) assetSources(c *gin.Context) {
	ok(c, gin.H{
		"sources":       modelhub.Sources(),
		"volume":        modelhub.VolumeName,
		"volume_target": modelhub.VolumeTarget,
		"pip_index":     "https://pypi.tuna.tsinghua.edu.cn/simple",
	})
}

func (h *Handler) listAssets(c *gin.Context) {
	assets, err := h.store.ListAssets()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"assets": assets})
}

func (h *Handler) createAsset(c *gin.Context) {
	var req modelhub.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		bad(c, err)
		return
	}
	asset, err := h.hub.Create(c.Request.Context(), req)
	if err != nil {
		h.audit(c, "下载模型", req.Name, err.Error(), "failed")
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(c, "下载模型", asset.Name, asset.Source+" "+asset.Repo, "ok")
	c.JSON(http.StatusAccepted, gin.H{"data": asset})
}

func (h *Handler) getAsset(c *gin.Context) {
	asset, err := h.store.GetAsset(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "模型资产不存在")
		return
	}
	ok(c, gin.H{"asset": asset})
}

func (h *Handler) assetLogs(c *gin.Context) {
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "300"))
	logs, err := h.hub.Logs(c.Request.Context(), c.Param("id"), tail)
	if err != nil {
		fail(c, http.StatusNotFound, err.Error())
		return
	}
	ok(c, gin.H{"logs": logs})
}

func (h *Handler) cancelAsset(c *gin.Context) {
	if err := h.hub.Cancel(c.Param("id")); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(c, "取消模型下载", c.Param("id"), "", "ok")
	ok(c, gin.H{"message": "已取消"})
}

func (h *Handler) scanAsset(c *gin.Context) {
	asset, err := h.hub.Scan(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"asset": asset})
}

func (h *Handler) deleteAsset(c *gin.Context) {
	deleteFiles := c.DefaultQuery("delete_files", "1") == "1"
	asset, err := h.store.GetAsset(c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "模型资产不存在")
		return
	}
	if err := h.hub.Remove(c.Request.Context(), asset.ID, deleteFiles); err != nil {
		h.audit(c, "删除模型资产", asset.Name, err.Error(), "failed")
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(c, "删除模型资产", asset.Name, "delete_files="+strconv.FormatBool(deleteFiles), "ok")
	ok(c, gin.H{"message": "已删除"})
}
