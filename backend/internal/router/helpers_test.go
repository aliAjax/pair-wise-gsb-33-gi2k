package router_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	gmserver "github.com/dolthub/go-mysql-server/server"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// startMemoryMySQL boots a pure-Go in-process MySQL-compatible server for
// end-to-end tests (no external database required).
func startMemoryMySQL(t *testing.T, addr string) {
	t.Helper()
	provider := memory.NewDBProvider(memory.NewDatabase("testdb"))
	engine := sqle.NewDefault(provider)
	srv, err := gmserver.NewDefaultServer(
		gmserver.Config{Protocol: "tcp", Address: addr},
		engine,
	)
	if err != nil {
		t.Fatalf("memory mysql server: %v", err)
	}
	go func() { _ = srv.Start() }()
	t.Cleanup(func() {
		_ = srv.Close()
	})
	waitForPort(t, addr)
}

func waitForPort(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("memory mysql at %s did not come up", addr)
}

// migrateModels creates the tables the article flow touches.
func migrateModels(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.User{},
		&model.CareArticle{},
		&model.ArticleRevision{},
	)
}

func seedTwoUsers(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	for _, u := range []struct{ username, email, password string }{
		{"owner", "owner@test.local", "owner123"},
		{"other", "other@test.local", "other123"},
	} {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			t.Fatal(err)
		}
		if err := gdb.Create(&model.User{
			Username: u.username, Email: u.email, PasswordHash: string(hash),
			Nickname: u.username, Role: "user",
		}).Error; err != nil {
			t.Fatalf("seed user %s: %v", u.username, err)
		}
	}
}

func login(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/login", &byteReader{data: body})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d %s", username, w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	return env.Data.Token
}

type byteReader struct {
	data []byte
	pos  int
}

func (b *byteReader) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
