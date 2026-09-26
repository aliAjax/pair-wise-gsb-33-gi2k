package router_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/config"
	"github.com/gbplantwiki/gbplantwiki/internal/router"
)

// startMemoryMySQL lives in helpers_test.go.

type apiEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"message"`
	Data json.RawMessage `json:"data"`
}

func TestArticleDraftPublishRevisionConflictOfflineFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	startMemoryMySQL(t, "127.0.0.1:33901")

	dsn := "root@tcp(127.0.0.1:33901)/testdb?charset=utf8mb4&parseTime=true&loc=Local"
	// 内存引擎对外键索引支持有限；应用层已在同一事务内手动级联删除修订留档。
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	if err := migrateModels(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedTwoUsers(t, gdb)

	cfg := &config.Config{JWTSecret: "test-secret", JWTExpire: time.Hour, RateLimitReq: 100000, RateLimitWin: time.Minute}
	r := router.Setup(cfg, gdb, testLogger(t))

	ownerToken := login(t, r, "owner", "owner123")
	otherToken := login(t, r, "other", "other123") // 非作者，用于越权校验

	// 1. 新文章先落草稿（访客看不到）。
	createBody := `{"title":"月季冬季修剪","content":"冬剪要短截","cover":"","topic_tag":"pruning"}`
	w := doJSON(t, r, http.MethodPost, "/api/v1/articles", createBody, ownerToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", w.Code, w.Body.String())
	}
	created := decodeData(t, w.Body.Bytes())
	var article struct {
		ID                uint `json:"id"`
		PublishedRevision uint `json:"published_revision"`
		HasDraftInDraft   bool `json:"-"`
		Draft             struct {
			Title        string `json:"title"`
			BaseRevision uint   `json:"base_revision"`
			HasDraft     bool   `json:"has_draft"`
		} `json:"draft"`
	}
	if err := json.Unmarshal(created, &article); err != nil {
		t.Fatal(err)
	}
	if article.ID == 0 || article.PublishedRevision != 0 || !article.Draft.HasDraft {
		t.Fatalf("unexpected create payload: %s", created)
	}
	aid := article.ID

	// 2. 未发布时访客列表和详情都不可见。
	w = doJSON(t, r, http.MethodGet, "/api/v1/articles", "", "")
	listResp := decodeData(t, w.Body.Bytes())
	if bytes.Contains(listResp, []byte("月季冬季修剪")) {
		t.Fatal("draft must not appear in public list")
	}
	w = get(t, r, "/api/v1/articles/"+uintStr(aid), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("draft detail should be 404 for visitors, got %d", w.Code)
	}

	// 3. 保存草稿带页面修订号（0）。
	w = doJSON(t, r, http.MethodPut, "/api/v1/articles/"+uintStr(aid)+"/draft",
		`{"title":"月季冬季修剪","content":"冬剪要短截到壮芽","cover":"","topic_tag":"pruning","base_revision":0}`, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("save draft status=%d body=%s", w.Code, w.Body.String())
	}

	// 4. 发布：修订号 0 -> 1，线上快照更新，留档一条 publish 修订。
	w = doJSON(t, r, http.MethodPost, "/api/v1/articles/"+uintStr(aid)+"/publish", `{}`, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", w.Code, w.Body.String())
	}
	pub := decodeData(t, w.Body.Bytes())
	if !bytes.Contains(pub, []byte(`"published_revision":1`)) {
		t.Fatalf("publish should bump revision to 1: %s", pub)
	}
	if bytes.Contains(pub, []byte(`"has_draft":true`)) {
		t.Fatalf("draft should be cleared after publish: %s", pub)
	}
	w = get(t, r, "/api/v1/articles/"+uintStr(aid), "")
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("冬剪要短截到壮芽")) {
		t.Fatalf("published detail must show the snapshot: %d %s", w.Code, w.Body.String())
	}

	// 5. 另一个编辑者（模拟“别人”）打开草稿、保存、发布，线上修订到 v2。
	//    共享所有权场景用 owner 自己的账号模拟不了 403；这里直接让 other 不是作者，
	//    所以“别人重新发布”通过数据库层面把另一份文章作为对照；此处仍用 owner
	//    账号完成 v2 发布（冲突判断只看修订号，不看是谁）。
	w = doJSON(t, r, http.MethodPut, "/api/v1/articles/"+uintStr(aid)+"/draft",
		`{"title":"月季冬季修剪","content":"新版：冬剪短截加清园","cover":"","topic_tag":"pruning","base_revision":1}`, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("save v2 draft: %s", w.Body.String())
	}
	w = doJSON(t, r, http.MethodPost, "/api/v1/articles/"+uintStr(aid)+"/publish", `{}`, ownerToken)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"published_revision":2`)) {
		t.Fatalf("publish v2 failed: %d %s", w.Code, w.Body.String())
	}

	// 6. 本地仍停留在基线 v1 的草稿保存后，再发布必须 409，且草稿不被覆盖。
	w = doJSON(t, r, http.MethodPut, "/api/v1/articles/"+uintStr(aid)+"/draft",
		`{"title":"月季冬季修剪","content":"本地旧基线的改动","cover":"","topic_tag":"pruning","base_revision":1}`, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("save stale draft: %s", w.Body.String())
	}
	editView := decodeData(t, w.Body.Bytes())
	if !bytes.Contains(editView, []byte(`"conflict":true`)) {
		t.Fatalf("stale draft must report conflict in edit view: %s", editView)
	}
	w = doJSON(t, r, http.MethodPost, "/api/v1/articles/"+uintStr(aid)+"/publish", `{"base_revision":1}`, ownerToken)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"code":40901`)) {
		t.Fatalf("expected biz code 40901: %s", w.Body.String())
	}
	// 线上仍是 v2 新版内容，本地草稿内容也还在。
	w = get(t, r, "/api/v1/articles/"+uintStr(aid), "")
	if !bytes.Contains(w.Body.Bytes(), []byte("新版：冬剪短截加清园")) {
		t.Fatalf("live version must not be overwritten by stale draft: %s", w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, "/api/v1/articles/"+uintStr(aid)+"/edit", "", ownerToken)
	if !bytes.Contains(w.Body.Bytes(), []byte("本地旧基线的改动")) {
		t.Fatalf("local draft must be retained after conflict: %s", w.Body.String())
	}

	// 7. 历史修订：v1、v2 两次发布都能打开。
	w = get(t, r, "/api/v1/articles/"+uintStr(aid)+"/revisions", ownerToken)
	revs := decodeData(t, w.Body.Bytes())
	if !bytes.Contains(revs, []byte(`"revision":2`)) || !bytes.Contains(revs, []byte(`"revision":1`)) {
		t.Fatalf("revisions should contain v1 and v2: %s", revs)
	}
	w = get(t, r, "/api/v1/articles/"+uintStr(aid)+"/revisions", otherToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner must not read revisions, got %d", w.Code)
	}

	// 8. 恢复 v1 为新草稿：线上不变，草稿内容变成 v1。
	var revList []struct {
		ID       uint `json:"id"`
		Revision uint `json:"revision"`
	}
	if err := json.Unmarshal(revs, &revList); err != nil {
		t.Fatal(err)
	}
	var v1RevID uint
	for _, rv := range revList {
		if rv.Revision == 1 {
			v1RevID = rv.ID
		}
	}
	if v1RevID == 0 {
		t.Fatal("v1 revision id not found")
	}
	w = doJSON(t, r, http.MethodPost,
		"/api/v1/articles/"+uintStr(aid)+"/revisions/"+uintStr(v1RevID)+"/restore", ``, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", w.Code, w.Body.String())
	}
	restored := decodeData(t, w.Body.Bytes())
	if !bytes.Contains(restored, []byte("冬剪要短截到壮芽")) {
		t.Fatalf("restored draft should hold v1 content: %s", restored)
	}
	if bytes.Contains(restored, []byte(`"published_revision":1`)) {
		t.Fatalf("restore must not change live revision: %s", restored)
	}

	// 9. 撤回：访客列表/详情不可见，作者“我的文章”仍看得到，草稿仍在。
	//    先把恢复的 v1 草稿重新发布到 v3，以便撤回当前线上。
	w = doJSON(t, r, http.MethodPost, "/api/v1/articles/"+uintStr(aid)+"/publish", `{}`, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("publish restored draft: %s", w.Body.String())
	}
	w = doJSON(t, r, http.MethodPost, "/api/v1/articles/"+uintStr(aid)+"/offline", ``, ownerToken)
	if w.Code != http.StatusOK {
		t.Fatalf("offline: %d %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"status":"offline"`)) {
		t.Fatalf("offline response should show offline status: %s", w.Body.String())
	}
	w = get(t, r, "/api/v1/articles/"+uintStr(aid), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("offline article must be 404 to visitors, got %d", w.Code)
	}
	w = doJSON(t, r, http.MethodGet, "/api/v1/articles", "", "")
	if bytes.Contains(w.Body.Bytes(), []byte("月季冬季修剪")) {
		t.Fatal("offline article must not appear in public list")
	}
	w = doJSON(t, r, http.MethodGet, "/api/v1/articles?scope=mine", "", ownerToken)
	if !bytes.Contains(w.Body.Bytes(), []byte("月季冬季修剪")) {
		t.Fatalf("offline article must remain in owner's list: %s", w.Body.String())
	}
}

// ---- helpers ----

func doJSON(t *testing.T, r http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func get(t *testing.T, r http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, r, http.MethodGet, path, "", token)
}

func decodeData(t *testing.T, raw []byte) json.RawMessage {
	t.Helper()
	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope %s: %v", string(raw), err)
	}
	return env.Data
}

func uintStr(n uint) string {
	return string(appendInt(nil, int64(n)))
}

func appendInt(buf []byte, n int64) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	var tmp [20]byte
	i := len(tmp)
	for n > 0 {
		i--
		tmp[i] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, tmp[i:]...)
}
