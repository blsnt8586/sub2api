package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type RadarFeedbackHandler struct{ service *service.RadarFeedbackService }

func NewRadarFeedbackHandler(svc *service.RadarFeedbackService) *RadarFeedbackHandler {
	return &RadarFeedbackHandler{service: svc}
}

func (h *RadarFeedbackHandler) ListMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, hasMore, err := h.service.ListMinePage(c, subject.UserID, limit, offset)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "has_more": hasMore})
}

func (h *RadarFeedbackHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := parseRadarFeedbackForm(c); err != nil {
		response.BadRequest(c, "invalid feedback form")
		return
	}
	attachments, err := parseRadarFeedbackAttachments(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Create(c, subject.UserID, c.PostForm("subject"), c.PostForm("content"), attachments)
	if err != nil {
		if errors.Is(err, service.ErrRadarFeedbackInvalid) {
			response.BadRequest(c, "invalid feedback content or attachment size")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RadarFeedbackHandler) UserReply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := parsePositiveID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid feedback id")
		return
	}
	if err = parseRadarFeedbackForm(c); err != nil {
		response.BadRequest(c, "invalid feedback form")
		return
	}
	attachments, err := parseRadarFeedbackAttachments(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.ReplyFromUser(c, id, subject.UserID, c.PostForm("content"), attachments)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRadarFeedbackInvalid):
			response.BadRequest(c, "invalid feedback content or attachment size")
		case errors.Is(err, service.ErrRadarFeedbackThreadMissing):
			response.NotFound(c, "feedback thread not found")
		case errors.Is(err, service.ErrRadarFeedbackThreadClosed):
			response.Error(c, http.StatusConflict, "feedback thread is closed")
		default:
			response.ErrorFrom(c, err)
		}
		return
	}
	response.Success(c, item)
}

func (h *RadarFeedbackHandler) AdminList(c *gin.Context) {
	items, err := h.service.ListAll(c, c.Query("status"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *RadarFeedbackHandler) AdminUsers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, more, err := h.service.AdminListUsers(c, subject.UserID, limit, offset)
	if feedbackAdminError(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "has_more": more})
}

func (h *RadarFeedbackHandler) AdminUserThreads(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, err := parsePositiveID(c.Param("user_id"))
	if feedbackAdminError(c, err) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, more, err := h.service.AdminListUserThreads(c, subject.UserID, id, c.Query("status"), limit, offset)
	if feedbackAdminError(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items, "has_more": more})
}

func (h *RadarFeedbackHandler) AdminThread(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, err := parsePositiveID(c.Param("id"))
	if feedbackAdminError(c, err) {
		return
	}
	item, err := h.service.AdminGetThread(c, subject.UserID, id)
	if feedbackAdminError(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *RadarFeedbackHandler) AdminRead(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, err := parsePositiveID(c.Param("id"))
	if feedbackAdminError(c, err) {
		return
	}
	var input struct {
		LastReadMessageID int64 `json:"last_read_message_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid read watermark")
		return
	}
	if feedbackAdminError(c, h.service.AdminMarkRead(c, subject.UserID, id, input.LastReadMessageID)) {
		return
	}
	response.Success(c, gin.H{"read": true})
}

func feedbackAdminError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrRadarFeedbackInvalid):
		response.BadRequest(c, "invalid feedback request")
	case errors.Is(err, service.ErrRadarFeedbackThreadMissing):
		response.NotFound(c, "feedback thread not found")
	default:
		response.ErrorFrom(c, err)
	}
	return true
}

func (h *RadarFeedbackHandler) AdminReply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return
	}
	id, err := parsePositiveID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid feedback id")
		return
	}
	if err = parseRadarFeedbackForm(c); err != nil {
		response.BadRequest(c, "invalid feedback form")
		return
	}
	attachments, err := parseRadarFeedbackAttachments(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Reply(c, id, subject.UserID, c.PostForm("content"), attachments, c.PostForm("close") == "true")
	if err != nil {
		if errors.Is(err, service.ErrRadarFeedbackInvalid) {
			response.BadRequest(c, "invalid feedback content or attachment size")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func parseRadarFeedbackForm(c *gin.Context) error {
	maxBytes := int64(service.RadarFeedbackMaxMessageImageBytes + service.RadarFeedbackMaxContentBytes + (1 << 20))
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	return c.Request.ParseMultipartForm(maxBytes)
}

func parseRadarFeedbackAttachments(c *gin.Context) ([]service.RadarFeedbackAttachmentInput, error) {
	if c.Request.MultipartForm == nil {
		return nil, service.ErrRadarFeedbackInvalid
	}
	files := c.Request.MultipartForm.File["attachments"]
	if len(files) > service.RadarFeedbackMaxAttachments {
		return nil, service.ErrRadarFeedbackInvalid
	}
	out := make([]service.RadarFeedbackAttachmentInput, 0, len(files))
	for _, header := range files {
		if header.Size <= 0 || header.Size > service.RadarFeedbackMaxImageBytes {
			return nil, service.ErrRadarFeedbackInvalid
		}
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, service.RadarFeedbackMaxImageBytes+1))
		file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if int64(len(data)) != header.Size {
			return nil, service.ErrRadarFeedbackInvalid
		}
		typ := strings.ToLower(strings.Split(http.DetectContentType(data), ";")[0])
		declared := strings.ToLower(strings.Split(header.Header.Get("Content-Type"), ";")[0])
		if declared != "" && declared != typ {
			return nil, service.ErrRadarFeedbackInvalid
		}
		out = append(out, service.RadarFeedbackAttachmentInput{Filename: header.Filename, ContentType: typ, SizeBytes: int64(len(data)), Content: data})
	}
	return out, nil
}

func parsePositiveID(raw string) (int64, error) {
	var id int64
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, service.ErrRadarFeedbackInvalid
		}
		id = id*10 + int64(r-'0')
		if id > 1<<62 {
			return 0, service.ErrRadarFeedbackInvalid
		}
	}
	if id <= 0 {
		return 0, service.ErrRadarFeedbackInvalid
	}
	return id, nil
}
