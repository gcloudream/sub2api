package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UserGroupAccountHandler 用户侧「账号管理」：只读查看自己被分配的专属分组下的上游账号。
type UserGroupAccountHandler struct {
	svc *service.UserGroupAccountService
}

func NewUserGroupAccountHandler(svc *service.UserGroupAccountService) *UserGroupAccountHandler {
	return &UserGroupAccountHandler{svc: svc}
}

// List GET /api/v1/user/group-accounts
func (h *UserGroupAccountHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	groups, err := h.svc.ListGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, groups)
}

// GetUsage GET /api/v1/user/group-accounts/:id/usage?source=passive|active
func (h *UserGroupAccountHandler) GetUsage(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	source := c.DefaultQuery("source", service.UserGroupAccountUsageSourceActive)
	usage, err := h.svc.GetUsage(c.Request.Context(), subject.UserID, accountID, source)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, usage)
}
