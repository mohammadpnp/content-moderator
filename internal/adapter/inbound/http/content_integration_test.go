package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http"
	"github.com/mohammadpnp/content-moderator/internal/adapter/outbound/postgres"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTest(t *testing.T) (*fiber.App, *postgres.ContentRepository, *sqlx.DB) {
	t.Helper()

	root, err := postgres.FindProjectRoot()
	require.NoError(t, err)
	envPath := filepath.Join(root, ".env")
	_ = godotenv.Load(envPath)

	db, err := postgres.NewDB()
	require.NoError(t, err)

	_, err = db.Exec("DELETE FROM contents")
	require.NoError(t, err)

	repo := postgres.NewContentRepository(db)
	broker := mock.NewMockMessageBroker()
	svc := service.NewContentService(repo, broker)

	app := fiber.New()
	http.SetupRoutes(app, svc)

	return app, repo, db
}

func TestIntegrationCreateContent(t *testing.T) {
	app, _, db := setupIntegrationTest(t)
	defer db.Close()

	t.Run("create text content", func(t *testing.T) {
		reqBody := `{"user_id":"user-integration","type":"text","body":"hello world"}`
		req := httptest.NewRequest("POST", "/api/v1/contents", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)

		require.NoError(t, err)
		assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

		var content entity.Content
		err = json.NewDecoder(resp.Body).Decode(&content)
		require.NoError(t, err)
		assert.Equal(t, "user-integration", content.UserID)
		assert.Equal(t, entity.ContentTypeText, content.Type)
		assert.NotEmpty(t, content.ID)
	})
}

func TestIntegrationGetContent(t *testing.T) {
	app, repo, db := setupIntegrationTest(t)
	defer db.Close()

	c, err := entity.NewContent("user-get", entity.ContentTypeText, "test get")
	require.NoError(t, err)
	contentID := uuid.New().String()
	c.ID = contentID
	err = repo.Save(context.Background(), c)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/contents/"+contentID, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var fetched entity.Content
	err = json.NewDecoder(resp.Body).Decode(&fetched)
	require.NoError(t, err)
	assert.Equal(t, "test get", fetched.Body)
}

func TestIntegrationListUserContents(t *testing.T) {
	app, repo, db := setupIntegrationTest(t)
	defer db.Close()

	for i := 0; i < 3; i++ {
		c, _ := entity.NewContent("user-list", entity.ContentTypeText, fmt.Sprintf("msg %d", i))
		c.ID = uuid.New().String()
		err := repo.Save(context.Background(), c)
		require.NoError(t, err)
	}

	req := httptest.NewRequest("GET", "/api/v1/users/user-list/contents?limit=10&offset=0", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	if resp.StatusCode != fiber.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	var contents []entity.Content
	err = json.NewDecoder(resp.Body).Decode(&contents)
	require.NoError(t, err)
	assert.Len(t, contents, 3)
}

func TestIntegrationDeleteContent(t *testing.T) {
	app, repo, db := setupIntegrationTest(t)
	defer db.Close()

	c, _ := entity.NewContent("user-del", entity.ContentTypeText, "to delete")
	c.ID = uuid.New().String()
	err := repo.Save(context.Background(), c)
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/contents/"+c.ID, nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)

	_, err = repo.FindByID(context.Background(), c.ID)
	assert.Error(t, err)
}
