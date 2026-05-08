package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockContentService struct {
	mock.Mock
}

func (m *MockContentService) CreateContent(ctx context.Context, userID string, ct entity.ContentType, body string) (*entity.Content, error) {
	args := m.Called(ctx, userID, ct, body)
	return args.Get(0).(*entity.Content), args.Error(1)
}

func (m *MockContentService) GetContent(ctx context.Context, id string) (*entity.Content, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*entity.Content), args.Error(1)
}

func (m *MockContentService) ListUserContents(ctx context.Context, userID string, limit, offset int) ([]*entity.Content, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]*entity.Content), args.Error(1)
}

func (m *MockContentService) DeleteContent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func setupTestApp() (*fiber.App, *MockContentService) {
	mockSvc := new(MockContentService)
	app := fiber.New()
	http.SetupRoutes(app, mockSvc)
	return app, mockSvc
}

func TestCreateContent_Success(t *testing.T) {
	app, mockSvc := setupTestApp()
	defer app.Shutdown()

	expected := &entity.Content{
		ID:     "123",
		UserID: "user1",
	}
	mockSvc.On("CreateContent", mock.Anything, "user1", entity.ContentTypeText, "Hello").
		Return(expected, nil)

	reqBody := `{"user_id":"user1","type":"text","body":"Hello"}`
	req := httptest.NewRequest("POST", "/api/v1/contents", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-user1")
	resp, err := app.Test(req, -1)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

	var result entity.Content
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, expected.ID, result.ID)
}

func TestCreateContent_ValidationError(t *testing.T) {
	app, _ := setupTestApp()
	defer app.Shutdown()

	reqBody := `{"user_id":"user1","type":"text","body":""}`
	req := httptest.NewRequest("POST", "/api/v1/contents", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-user1")
	resp, _ := app.Test(req, -1)

	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestGetContent_Success(t *testing.T) {
	app, mockSvc := setupTestApp()
	defer app.Shutdown()

	content := &entity.Content{ID: "abc", Body: "sample"}
	mockSvc.On("GetContent", mock.Anything, "abc").Return(content, nil)

	req := httptest.NewRequest("GET", "/api/v1/contents/abc", nil)
	req.Header.Set("Authorization", "Bearer valid-user1")
	resp, _ := app.Test(req, -1)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestGetContent_NotFound(t *testing.T) {
	app, mockSvc := setupTestApp()
	defer app.Shutdown()

	mockSvc.On("GetContent", mock.Anything, "xyz").Return(
		(*entity.Content)(nil), errors.New("not found"),
	)

	req := httptest.NewRequest("GET", "/api/v1/contents/xyz", nil)
	req.Header.Set("Authorization", "Bearer valid-user1")
	resp, _ := app.Test(req, -1)

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
