package security

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/pkg/dto"
)

func newRegistryTestService(t *testing.T) (*RegistryService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:registry_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.ImageRegistry{}); err != nil {
		t.Fatalf("auto migrate image_registry failed: %v", err)
	}
	return NewRegistryService(db), db
}

func TestRegistryServiceCreateEncryptsPassword(t *testing.T) {
	svc, db := newRegistryTestService(t)

	req := &dto.ImageRegistryRequest{
		Name:     "test-registry",
		Type:     "harbor",
		URL:      "https://registry.example.com",
		Username: "alice",
		Password: "plain-secret",
	}
	if err := svc.Create(context.Background(), req); err != nil {
		t.Fatalf("create registry failed: %v", err)
	}

	var saved models.ImageRegistry
	if err := db.First(&saved).Error; err != nil {
		t.Fatalf("query saved registry failed: %v", err)
	}
	if saved.Password == "" || saved.Password == req.Password {
		t.Fatalf("password should be encrypted, got %q", saved.Password)
	}
	decrypted, err := decryptRegistryPassword(saved.Password)
	if err != nil {
		t.Fatalf("decrypt saved password failed: %v", err)
	}
	if decrypted != req.Password {
		t.Fatalf("password roundtrip mismatch, got %q want %q", decrypted, req.Password)
	}
}

func TestResolveRegistryPasswordFallbackToLegacyPlaintext(t *testing.T) {
	svc, db := newRegistryTestService(t)

	reg := &models.ImageRegistry{
		Name:     "legacy",
		Type:     "harbor",
		URL:      "https://legacy-registry.example.com",
		Username: "legacy-user",
		Password: "legacy-plaintext-password",
	}
	if err := db.Create(reg).Error; err != nil {
		t.Fatalf("create registry failed: %v", err)
	}

	got, err := svc.resolveRegistryPassword(context.Background(), &dto.ImageRegistryTestRequest{
		RegistryID: reg.ID,
	})
	if err != nil {
		t.Fatalf("resolve password failed: %v", err)
	}
	if got != reg.Password {
		t.Fatalf("expected legacy plaintext fallback, got %q want %q", got, reg.Password)
	}
}

func TestTestConnectionUsesStoredPasswordByRegistryID(t *testing.T) {
	svc, db := newRegistryTestService(t)

	expectedUser := "stored-user"
	expectedPass := "stored-pass"
	encPass, err := encryptRegistryPassword(expectedPass)
	if err != nil {
		t.Fatalf("encrypt stored password failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(expectedUser+":"+expectedPass))
		if r.Header.Get("Authorization") != wantAuth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer server.Close()

	reg := &models.ImageRegistry{
		Name:     "stored",
		Type:     "docker",
		URL:      server.URL,
		Username: expectedUser,
		Password: encPass,
	}
	if err := db.Create(reg).Error; err != nil {
		t.Fatalf("create registry failed: %v", err)
	}

	err = svc.TestConnection(context.Background(), &dto.ImageRegistryTestRequest{
		RegistryID: reg.ID,
		Type:       "docker",
		URL:        server.URL,
		Username:   expectedUser,
		Password:   "",
	})
	if err != nil {
		t.Fatalf("test connection should use stored password, got error: %v", err)
	}
}
