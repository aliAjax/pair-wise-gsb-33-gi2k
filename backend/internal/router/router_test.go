package router

import (
	"io"
	"log/slog"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/config"
)

// TestSetupRegistersArticleRoutes ensures the public and /account editing
// route trees coexist without Gin path-parameter conflicts.
func TestSetupRegistersArticleRoutes(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	r := Setup(config.Load(), db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	want := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/articles"},
		{"GET", "/api/v1/articles/:id"},
		{"GET", "/api/v1/account/articles"},
		{"POST", "/api/v1/account/articles/publish"},
		{"POST", "/api/v1/account/articles/:id/withdraw"},
		{"GET", "/api/v1/account/articles/:id/revisions"},
		{"GET", "/api/v1/account/articles/:id/revisions/:no"},
		{"POST", "/api/v1/account/articles/:id/revisions/:no/restore"},
		{"GET", "/api/v1/account/drafts"},
		{"GET", "/api/v1/account/drafts/open"},
		{"GET", "/api/v1/account/drafts/:id"},
		{"PUT", "/api/v1/account/drafts"},
		{"POST", "/api/v1/account/drafts/:id/reconcile"},
		{"DELETE", "/api/v1/account/drafts/:id"},
	}
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, w := range want {
		if !registered[w.method+" "+w.path] {
			t.Errorf("route not registered: %s %s", w.method, w.path)
		}
	}
}
