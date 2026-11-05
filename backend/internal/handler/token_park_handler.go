package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TokenParkHandler struct {
	rewards *service.TokenParkRewardService
}

func NewTokenParkHandler(rewards *service.TokenParkRewardService) *TokenParkHandler {
	return &TokenParkHandler{rewards: rewards}
}

func (h *TokenParkHandler) Status(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	status, err := h.rewards.Status(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *TokenParkHandler) Leaderboard(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	leaderboard, err := h.rewards.Leaderboard(c.Request.Context(), subject.UserID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrTokenParkDisabled) || errors.Is(err, service.ErrTokenParkNotEligible) {
			status = http.StatusForbidden
		}
		response.Error(c, status, err.Error())
		return
	}
	response.Success(c, leaderboard)
}

func (h *TokenParkHandler) AdminLeaderboard(c *gin.Context) {
	leaderboard, err := h.rewards.AdminLeaderboard(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, leaderboard)
}

func (h *TokenParkHandler) Claim(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	game := strings.TrimSpace(c.Param("game"))
	var req struct {
		Proof service.TokenParkProof `json:"proof"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid proof")
		return
	}
	amount, balance, err := h.rewards.Claim(c.Request.Context(), subject.UserID, game, req.Proof)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, service.ErrTokenParkDisabled):
			status = http.StatusForbidden
		case errors.Is(err, service.ErrTokenParkNotEligible):
			status = http.StatusForbidden
		case errors.Is(err, service.ErrTokenParkAlreadyClaimed):
			status = http.StatusConflict
		}
		response.Error(c, status, err.Error())
		return
	}
	response.Success(c, gin.H{"game": game, "amount_usd": amount, "balance": balance, "claimed": true})
}

func (h *TokenParkHandler) GetProgress(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	progress, err := h.rewards.LoadProgress(c.Request.Context(), subject.UserID, c.Param("game"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, progress)
}

func (h *TokenParkHandler) SaveProgress(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req struct {
		State json.RawMessage `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.State) == 0 || !json.Valid(req.State) {
		response.Error(c, http.StatusBadRequest, "invalid progress state")
		return
	}
	progress, err := h.rewards.SaveProgress(c.Request.Context(), subject.UserID, c.Param("game"), string(req.State))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, progress)
}

func (h *TokenParkHandler) DeleteProgress(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.rewards.DeleteProgress(c.Request.Context(), subject.UserID, c.Param("game")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
