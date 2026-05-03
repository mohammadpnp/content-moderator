package http

import (
	"net/http"
	"strconv"

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
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
			"code":  http.StatusBadRequest,
		})
	}

	contentType := entity.ContentType(req.Type)
	if err := contentType.Validate(); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
			"code":  http.StatusBadRequest,
		})
	}

	content, err := h.service.CreateContent(c.UserContext(), req.UserID, contentType, req.Body)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
			"code":  http.StatusInternalServerError,
		})
	}

	return c.Status(http.StatusCreated).JSON(content)
}

func (h *ContentHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	content, err := h.service.GetContent(c.UserContext(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "content not found",
			"code":  http.StatusNotFound,
		})
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
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
			"code":  http.StatusInternalServerError,
		})
	}
	return c.JSON(contents)
}

func (h *ContentHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	err := h.service.DeleteContent(c.UserContext(), id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "content not found",
			"code":  http.StatusNotFound,
		})
	}
	return c.SendStatus(http.StatusNoContent)
}
