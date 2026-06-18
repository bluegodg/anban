package childapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bluegodg/anban/server/internal/domains/account"
	"github.com/bluegodg/anban/server/internal/domains/devicebinding"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
	"github.com/gin-gonic/gin"
)

func registerAccountRoutes(api *gin.RouterGroup, accountService *account.Service, bindingService *devicebinding.Service) {
	api.POST("/auth/register", func(c *gin.Context) {
		var req account.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
		resp, err := accountService.Register(c.Request.Context(), req)
		if errors.Is(err, account.ErrDuplicatePhone) {
			c.JSON(http.StatusConflict, gin.H{"error": "phone_already_registered"})
			return
		}
		if errors.Is(err, account.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "手机号和至少 6 位密码必填"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
			return
		}
		c.JSON(http.StatusCreated, resp)
	})

	api.POST("/auth/login", func(c *gin.Context) {
		var req account.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
		resp, err := accountService.Login(c.Request.Context(), req)
		if errors.Is(err, account.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "手机号或密码错误"})
			return
		}
		if errors.Is(err, account.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "手机号和密码必填"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败"})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	api.POST("/auth/verification-code", func(c *gin.Context) {
		var req account.VerificationCodeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
		resp, err := accountService.SendVerificationCode(c.Request.Context(), req)
		if errors.Is(err, account.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "手机号必填"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码发送失败"})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	api.POST("/auth/code-login", func(c *gin.Context) {
		var req account.CodeLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
		resp, err := accountService.CodeLogin(c.Request.Context(), req)
		if errors.Is(err, account.ErrInvalidVerificationCode) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码无效或已过期"})
			return
		}
		if errors.Is(err, account.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "手机号和验证码必填"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码登录失败"})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	protected := api.Group("", RequireBearerAccount(accountService))
	protected.POST("/auth/logout", func(c *gin.Context) {
		if err := accountService.Logout(c.Request.Context(), c.GetHeader("Authorization")); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	protected.GET("/me", func(c *gin.Context) {
		accountID := contextAccountID(c)
		publicAccount, err := accountService.GetAccount(c.Request.Context(), accountID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "账号读取失败"})
			return
		}
		var binding any
		if bindingService != nil {
			current, err := bindingService.CurrentBinding(c.Request.Context(), accountID)
			if err == nil {
				binding = current
			} else if !errors.Is(err, devicebinding.ErrNotBound) {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "绑定读取失败"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"account": publicAccount, "binding": binding})
	})
	protected.PUT("/me", func(c *gin.Context) {
		var req account.UpdateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
		publicAccount, err := accountService.UpdateProfile(c.Request.Context(), contextAccountID(c), req)
		if errors.Is(err, account.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "账号资料无效"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "账号资料保存失败"})
			return
		}
		c.JSON(http.StatusOK, publicAccount)
	})

	if bindingService == nil {
		return
	}
	protected.POST("/device-binding", func(c *gin.Context) {
		var req devicebinding.BindRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
			return
		}
		req.AccountID = contextAccountID(c)
		binding, err := bindingService.Bind(c.Request.Context(), req)
		switch {
		case errors.Is(err, devicebinding.ErrDeviceNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "device_code_not_found"})
		case errors.Is(err, devicebinding.ErrAccountAlreadyBound):
			c.JSON(http.StatusConflict, gin.H{"error": "account_already_bound"})
		case errors.Is(err, devicebinding.ErrAdminAlreadyBound):
			c.JSON(http.StatusConflict, gin.H{"error": "admin_already_bound"})
		case errors.Is(err, devicebinding.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": "role 和 bindingCode 必填"})
		case err != nil:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "设备绑定失败"})
		default:
			c.JSON(http.StatusCreated, binding)
		}
	})
	protected.DELETE("/device-binding", func(c *gin.Context) {
		if err := bindingService.UnbindAdmin(c.Request.Context(), contextAccountID(c)); err != nil {
			writeBindingManageError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	protected.POST("/device-binding/reset-code", func(c *gin.Context) {
		result, err := bindingService.ResetBindingCode(c.Request.Context(), contextAccountID(c))
		if err != nil {
			writeBindingManageError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
	protected.GET("/device-binding/members", func(c *gin.Context) {
		members, err := bindingService.ListMembers(c.Request.Context(), contextAccountID(c))
		if err != nil {
			writeBindingManageError(c, err)
			return
		}
		out := make([]gin.H, 0, len(members))
		for _, member := range members {
			publicAccount, err := accountService.GetAccount(c.Request.Context(), member.AccountID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "成员账号读取失败"})
				return
			}
			out = append(out, gin.H{
				"accountId":           member.AccountID,
				"bindingId":           member.BindingID,
				"role":                member.Role,
				"boundAt":             member.BoundAt,
				"phone":               publicAccount.Phone,
				"nickname":            publicAccount.Nickname,
				"displayName":         publicAccount.DisplayName,
				"realName":            publicAccount.RealName,
				"relationshipToElder": publicAccount.RelationshipToElder,
				"avatarColor":         publicAccount.AvatarColor,
			})
		}
		c.JSON(http.StatusOK, gin.H{"members": out})
	})
	protected.DELETE("/device-binding/members/:accountId", func(c *gin.Context) {
		memberID64, err := strconv.ParseUint(c.Param("accountId"), 10, 64)
		if err != nil || memberID64 == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "accountId 无效"})
			return
		}
		if err := bindingService.RemoveMember(c.Request.Context(), contextAccountID(c), uint(memberID64)); err != nil {
			writeBindingManageError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

func contextAccountID(c *gin.Context) uint {
	value, ok := c.Get(sharedtypes.GinContextAccountID)
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case uint:
		return v
	case int:
		return uint(v)
	default:
		return 0
	}
}

func writeBindingManageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, devicebinding.ErrAdminRequired):
		c.JSON(http.StatusForbidden, gin.H{"error": "admin_required"})
	case errors.Is(err, devicebinding.ErrNotBound):
		c.JSON(http.StatusConflict, gin.H{"error": "device_not_bound", "message": "请先绑定安伴设备"})
	case errors.Is(err, devicebinding.ErrMemberNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "member_not_found"})
	case errors.Is(err, devicebinding.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设备绑定管理失败"})
	}
}
