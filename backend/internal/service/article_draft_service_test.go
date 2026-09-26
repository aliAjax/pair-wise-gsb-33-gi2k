package service

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

func tTime() time.Time { return time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC) }

func newDraftServiceDB(t *testing.T) (*gorm.DB, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
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
	return db, sqlDB, mock
}

func newDraftService(t *testing.T) (*ArticleDraftService, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, sqlDB, mock := newDraftServiceDB(t)
	artRepo := repository.NewCareArticleRepository(db)
	draftRepo := repository.NewArticleDraftRepository(db)
	revRepo := repository.NewArticleRevisionRepository(db)
	svc := NewArticleDraftService(db, artRepo, draftRepo, revRepo, newTestLogger())
	return svc, sqlDB, mock
}

func articleColumns() []string {
	return []string{"id", "user_id", "title", "content", "cover", "topic_tag", "status", "revision_no", "view_count", "created_at", "updated_at"}
}

func draftColumns() []string {
	return []string{"id", "article_id", "user_id", "title", "content", "cover", "topic_tag", "base_revision_no", "saved_at", "created_at", "updated_at"}
}

func revisionColumns() []string {
	return []string{"id", "article_id", "revision_no", "user_id", "title", "content", "cover", "topic_tag", "summary", "created_at"}
}

// Publishing a draft based on the current online revision must append a
// revision snapshot, advance the article revision and delete the draft in
// one transaction.
func TestArticleDraftPublishHappyPath(t *testing.T) {
	svc, sqlDB, mock := newDraftService(t)
	defer sqlDB.Close()

	// FindByID draft
	mock.ExpectQuery("SELECT \\* FROM `article_drafts`").
		WithArgs(uint64(7), uint64(9), 1).
		WillReturnRows(sqlmock.NewRows(draftColumns()).AddRow(
			7, 9, 9, "新标题", "新正文", "", "pruning", 3, tTime(), tTime(), tTime()))
	// loadOwnedArticle
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WithArgs(uint64(9), 1).
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "旧标题", "旧正文", "", "pruning", "published", 3, 10, tTime(), tTime()))
	mock.ExpectBegin()
	// locked re-read inside tx
	mock.ExpectQuery("SELECT \\* FROM `care_articles`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "旧标题", "旧正文", "", "pruning", "published", 3, 10, tTime(), tTime()))
	mock.ExpectExec("INSERT INTO `article_revisions`").WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectExec("UPDATE `care_articles`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `article_drafts`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	out, err := svc.Publish(9, dto.ArticlePublishRequest{DraftID: 7})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if out.RevisionNo != 4 {
		t.Errorf("revision_no = %d, want 4", out.RevisionNo)
	}
	if out.Title != "新标题" || out.Status != constants.ArticleStatusPublished {
		t.Errorf("unexpected online snapshot: %+v", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

// Someone else (technically another session of the owner) republished
// while the draft was open: publish must be rejected with code 40901 and
// the draft must be left untouched.
func TestArticleDraftPublishStaleRejected(t *testing.T) {
	svc, sqlDB, mock := newDraftService(t)
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT \\* FROM `article_drafts`").
		WillReturnRows(sqlmock.NewRows(draftColumns()).AddRow(
			7, 9, 9, "本地草稿", "本地正文", "", "pruning", 3, tTime(), tTime(), tTime()))
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "线上标题", "线上正文", "", "pruning", "published", 4, 11, tTime(), tTime()))
	mock.ExpectBegin()
	// locked re-read returns the newer revision 4 -> stale
	mock.ExpectQuery("SELECT \\* FROM `care_articles`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "线上标题", "线上正文", "", "pruning", "published", 4, 11, tTime(), tTime()))
	mock.ExpectRollback()
	// fresh reload for the conflict details
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "线上标题", "线上正文", "", "pruning", "published", 4, 11, tTime(), tTime()))

	_, err := svc.Publish(9, dto.ArticlePublishRequest{DraftID: 7})
	var appErr *util.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want AppError, got %v", err)
	}
	if appErr.HTTPStatus != 409 || appErr.Code != constants.CodeRevisionStale {
		t.Errorf("got status=%d code=%d, want 409/%d", appErr.HTTPStatus, appErr.Code, constants.CodeRevisionStale)
	}
	details, ok := appErr.Details.(dto.RevisionConflictDetails)
	if !ok {
		t.Fatalf("want RevisionConflictDetails, got %T", appErr.Details)
	}
	if details.BaseRevisionNo != 3 || details.OnlineRevisionNo != 4 || details.ArticleID != 9 || details.DraftID != 7 {
		t.Errorf("unexpected conflict details: %+v", details)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

// A draft opened against a withdrawn article can still be edited, and
// publishing it brings the article back online; the optimistic guard is
// unchanged regardless of online status.
func TestArticleDraftSaveAgainstWithdrawnSucceeds(t *testing.T) {
	svc, sqlDB, mock := newDraftService(t)
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "已撤回", "正文", "", "pruning", constants.ArticleStatusWithdrawn, 3, 1, tTime(), tTime()))
	mock.ExpectQuery("SELECT \\* FROM `article_drafts`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `article_drafts`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()

	resp, err := svc.Save(9, dto.ArticleDraftSaveRequest{
		ArticleID: 9, Title: "改稿", Content: "新正文", TopicTag: "pruning", BaseRevisionNo: 3,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if resp.ArticleStatus != constants.ArticleStatusWithdrawn || resp.DraftID != 7 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// Saving into an existing draft never moves the frozen base revision.
func TestArticleDraftSaveKeepsBaseRevision(t *testing.T) {
	svc, sqlDB, mock := newDraftService(t)
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT \\* FROM `article_drafts`").
		WillReturnRows(sqlmock.NewRows(draftColumns()).AddRow(
			7, 9, 9, "标题", "正文", "", "pruning", 2, tTime(), tTime(), tTime()))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `article_drafts`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// reload article for response meta
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "线上", "线上", "", "pruning", "published", 3, 1, tTime(), tTime()))

	resp, err := svc.Save(9, dto.ArticleDraftSaveRequest{DraftID: 7, Title: "新标题", Content: "新正文", TopicTag: "pruning", BaseRevisionNo: 99})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if resp.BaseRevisionNo != 2 {
		t.Errorf("base revision moved to %d, want frozen 2", resp.BaseRevisionNo)
	}
	if !resp.Conflict || resp.OnlineRevisionNo != 3 {
		t.Errorf("expected conflict against online revision 3, got %+v", resp)
	}
}

// Restoring a revision while another draft exists must be refused so the
// unpublished draft cannot be overwritten.
func TestArticleRestoreBlockedByExistingDraft(t *testing.T) {
	svc, sqlDB, mock := newDraftService(t)
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "线上", "线上", "", "pruning", "published", 3, 1, tTime(), tTime()))
	mock.ExpectQuery("SELECT \\* FROM `article_revisions`").
		WillReturnRows(sqlmock.NewRows(revisionColumns()).AddRow(
			20, 9, 1, 9, "历史标题", "历史正文", "", "pruning", "", tTime()))
	mock.ExpectQuery("SELECT \\* FROM `article_drafts`").
		WillReturnRows(sqlmock.NewRows(draftColumns()).AddRow(
			7, 9, 9, "未发布草稿", "正文", "", "pruning", 3, tTime(), tTime(), tTime()))

	_, err := svc.Restore(9, 9, 1)
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 409 {
		t.Fatalf("want 409 AppError, got %v", err)
	}
}

// Withdrawn flips only the status column and keeps content/revisions.
func TestArticleWithdraw(t *testing.T) {
	svc, sqlDB, mock := newDraftService(t)
	defer sqlDB.Close()

	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id`").
		WillReturnRows(sqlmock.NewRows(articleColumns()).AddRow(
			9, 9, "线上", "线上", "", "pruning", "published", 3, 1, tTime(), tTime()))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `care_articles` SET `status`=? WHERE id = ?")).
		WithArgs(constants.ArticleStatusWithdrawn, uint64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.Withdraw(9, 9); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
}
