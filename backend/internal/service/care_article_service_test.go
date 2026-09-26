package service

import (
	"io"
	"log/slog"
	"testing"
	"time"

	sqlmock2 "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

func newArticleMockDB(t *testing.T) (*gorm.DB, sqlmock2.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock2.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db, mock
}

func articleServiceWithMock(db *gorm.DB) *CareArticleService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewCareArticleService(repository.NewCareArticleRepository(db), repository.NewUserRepository(db), logger)
}

func ownerArticleRows(rev, base uint, hasDraft bool, status string) *sqlmock2.Rows {
	now := time.Now()
	return sqlmock2.NewRows([]string{
		"id", "user_id", "title", "content", "cover", "topic_tag", "status", "view_count",
		"created_at", "updated_at", "published_revision", "published_at",
		"draft_title", "draft_content", "draft_cover", "draft_topic_tag",
		"draft_base_revision", "has_draft", "draft_saved_at",
	}).AddRow(
		uint64(7), uint64(1), "线上标题", "线上正文", "", "pruning", status, 10,
		now, now, rev, now,
		"草稿标题", "草稿正文", "", "pruning",
		base, hasDraft, now,
	)
}

// 线上已被重新发布时发布必须失败为 409 修订冲突，且不能有任何写操作（草稿保留）。
func TestCareArticleServicePublishConflict(t *testing.T) {
	db, mock := newArticleMockDB(t)
	svc := articleServiceWithMock(db)

	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(ownerArticleRows(3, 2, true, "published"))

	_, err := svc.Publish(7, 1, dto.ArticlePublishRequest{})
	appErr, ok := err.(*util.AppError)
	if !ok {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.HTTPStatus != 409 || appErr.Code != constants.CodeRevisionConflict {
		t.Fatalf("expected 409/%d, got %d/%d", constants.CodeRevisionConflict, appErr.HTTPStatus, appErr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 基线一致时发布进入仓储（这里模拟仓储 CAS 仍然失败，也必须映射成 409）。
func TestCareArticleServicePublishRepoConflictMapped(t *testing.T) {
	db, mock := newArticleMockDB(t)
	svc := articleServiceWithMock(db)

	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(ownerArticleRows(3, 3, true, "published"))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(ownerArticleRows(4, 3, true, "published")) // 事务内线上已被重新发布到修订 4
	mock.ExpectRollback()

	_, err := svc.Publish(7, 1, dto.ArticlePublishRequest{})
	appErr, ok := err.(*util.AppError)
	if !ok {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.Code != constants.CodeRevisionConflict {
		t.Fatalf("expected code %d, got %d", constants.CodeRevisionConflict, appErr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildOwnerViewSeedsDraftFromLive(t *testing.T) {
	a := &model.CareArticle{
		ID: 1, Title: "线上", Content: "正文", TopicTag: constants.TopicTagPruning,
		Status: constants.ArticleStatusPublished, PublishedRevision: 4, HasDraft: false,
	}
	v := buildOwnerView(a)
	if v.Draft.HasDraft {
		t.Fatal("new editor copy should not be marked as a saved draft")
	}
	if v.Draft.Title != "线上" || v.Draft.BaseRevision != 4 {
		t.Fatalf("draft should be seeded from live snapshot, got %+v", v.Draft)
	}
	if v.Conflict {
		t.Fatal("no saved draft means no conflict")
	}
}

func TestBuildOwnerViewFlagsStaleDraft(t *testing.T) {
	a := &model.CareArticle{
		ID: 1, Title: "线上新版", PublishedRevision: 5,
		HasDraft: true, DraftTitle: "本地草稿", DraftBaseRevision: 4,
	}
	v := buildOwnerView(a)
	if !v.Conflict {
		t.Fatal("expected conflict when draft base revision is behind live")
	}
	if v.Draft.Title != "本地草稿" {
		t.Fatalf("local draft content must be retained, got %q", v.Draft.Title)
	}
}

func TestBuildOwnerViewNoConflictWhenBaseCurrent(t *testing.T) {
	a := &model.CareArticle{
		ID: 1, PublishedRevision: 5,
		HasDraft: true, DraftTitle: "本地草稿", DraftBaseRevision: 5,
	}
	if buildOwnerView(a).Conflict {
		t.Fatal("base revision matching live revision must not be a conflict")
	}
}
