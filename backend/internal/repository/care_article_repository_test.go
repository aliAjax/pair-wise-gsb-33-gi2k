package repository

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func articleRows(aid, userID, liveRevision, baseRevision uint, hasDraft bool, status string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "user_id", "title", "content", "cover", "topic_tag", "status", "view_count",
		"created_at", "updated_at", "published_revision", "published_at",
		"draft_title", "draft_content", "draft_cover", "draft_topic_tag",
		"draft_base_revision", "has_draft", "draft_saved_at",
	}).AddRow(
		aid, userID, "线上标题", "线上正文", "", "pruning", status, 10,
		now, now, liveRevision, now,
		"草稿标题", "草稿正文", "", "pruning",
		baseRevision, hasDraft, now,
	)
}

// 草稿基线与线上修订号一致：发布成功，修订号 +1 并留档，草稿清空。
func TestCareArticleRepositoryPublishDraftSuccess(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareArticleRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(articleRows(7, 1, 2, 2, true, "published"))
	mock.ExpectExec("UPDATE `care_articles` SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `article_revisions`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(articleRows(7, 1, 3, 2, false, "published"))
	mock.ExpectCommit()

	out, err := repo.PublishDraft(7, 2, 1, "绿手指")
	if err != nil {
		t.Fatalf("PublishDraft expected success, got %v", err)
	}
	if out.PublishedRevision != 3 {
		t.Fatalf("expected new revision 3, got %d", out.PublishedRevision)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 线上已被别人重新发布（修订号领先草稿基线）：返回冲突，不执行 UPDATE/INSERT，草稿保留。
func TestCareArticleRepositoryPublishDraftStaleConflict(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareArticleRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(articleRows(7, 1, 3, 2, true, "published"))
	mock.ExpectRollback()

	_, err := repo.PublishDraft(7, 2, 1, "绿手指")
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 没有草稿时发布同样被拒绝，避免误覆盖线上。
func TestCareArticleRepositoryPublishDraftWithoutDraft(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareArticleRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `care_articles` WHERE `care_articles`.`id` = \\?").
		WithArgs(uint64(7), 1).
		WillReturnRows(articleRows(7, 1, 2, 2, false, "published"))
	mock.ExpectRollback()

	_, err := repo.PublishDraft(7, 2, 1, "绿手指")
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
