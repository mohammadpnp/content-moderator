package http

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
)

type ContentHandler struct {
	service inbound.ContentService
}

func NewContentHandler(svc inbound.ContentService) *ContentHandler {
	return &ContentHandler{service: svc}
}

type CreateContentRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Type   string `json:"type" validate:"required,oneof=text image"`
	Body   string `json:"body" validate:"required,max=10000"`
}

func (h *ContentHandler) Create(c *fiber.Ctx) error {
	var req CreateContentRequest
	if err := c.BodyParser(&req); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, "invalid request body")
	}

	userID, ok := c.Locals("userID").(string)
	if !ok || userID == "" {
		return errorResponse(c, fiber.StatusUnauthorized, "user not authenticated")
	}
	req.UserID = userID

	if err := validate.Struct(req); err != nil {
		return errorResponse(c, fiber.StatusBadRequest, formatValidationError(err))
	}

	contentType := entity.ContentType(req.Type)
	content, err := h.service.CreateContent(c.UserContext(), req.UserID, contentType, req.Body)
	if err != nil {
		return errorResponse(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(content)
}

func (h *ContentHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	content, err := h.service.GetContent(c.UserContext(), id)
	if err != nil {
		return errorResponse(c, fiber.StatusNotFound, "content not found")
	}
	return c.JSON(content)
}

func (h *ContentHandler) List(c *fiber.Ctx) error {
	userID := c.Params("userID")
	limitStr := c.Query("limit", "20")
	offsetStr := c.Query("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	contents, err := h.service.ListUserContents(c.UserContext(), userID, limit, offset)
	if err != nil {
		return errorResponse(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(contents)
}

func (h *ContentHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	err := h.service.DeleteContent(c.UserContext(), id)
	if err != nil {
		return errorResponse(c, fiber.StatusNotFound, "content not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// helper functions

func errorResponse(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error":    message,
		"code":     status,
		"trace_id": c.Locals("requestid"),
	})
}

func formatValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var msgs []string
		for _, e := range validationErrors {
			msgs = append(msgs, fmt.Sprintf("field '%s' failed on '%s'", e.Field(), e.Tag()))
		}
		return strings.Join(msgs, "; ")
	}
	return err.Error()
}
