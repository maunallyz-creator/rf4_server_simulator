package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"rf4server/internal/admin"
	"rf4server/internal/config"
	"rf4server/internal/logging"
	"rf4server/internal/store"
	"rf4server/internal/supervisor"
)

func TestHandleAdminNotReady(t *testing.T) {
	a := &app{}
	rr := httptest.NewRecorder()
	a.handleAdmin(rr, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rr.Body.String(), "管理后台尚未就绪") {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestHandleAdminServesEmbeddedControlDeck(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv := admin.NewServer(config.Admin{JWTSecret: "test"}, db, logging.Discard(), admin.WebFS())
	a := &app{adminSrv: srv}

	rr := httptest.NewRecorder()
	a.handleAdmin(rr, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "RF4 管理后台") {
		t.Fatalf("admin page: code=%d body=%q", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	a.handleAdmin(rr, httptest.NewRequest(http.MethodGet, "/admin/api/me", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("admin api: code=%d body=%q", rr.Code, rr.Body.String())
	}
}

// 托盘初始化（systray.Run）之前，子进程状态回调可能已经触发；
// 此时不得调用 systray，否则会因内部 winTray 为 nil 而 panic 并带走 launcher。
func TestUpdateTrayIconBeforeTrayReady(t *testing.T) {
	a := &app{sup: supervisor.New(t.TempDir(), "config.yaml", "")}
	updateTrayIcon(a)
	if a.trayColor != 2 {
		t.Fatalf("trayColor=%d, want 2（已停止）", a.trayColor)
	}
	if a.trayReady {
		t.Fatal("trayReady 不应在 systray.Run 之前被置位")
	}
}

// 控制中心首页的「后台信息」卡读 /api/overview：库没就绪时只能报 ready=false，
// 就绪后必须给出主机、账号与四类计数，否则首页会一直显示占位符。
func TestOverviewReportsAdminCounts(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "overview.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	a := &app{cfg: &config.Config{Admin: config.Admin{Username: "admin"}}, adminDB: db}
	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	req.Host = "127.0.0.1:9500"
	rr := httptest.NewRecorder()
	a.handleOverview(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d, want %d", rr.Code, http.StatusOK)
	}
	var out struct {
		Admin     string `json:"admin"`
		AdminUser string `json:"admin_user"`
		Host      string `json:"host"`
		Ready     bool   `json:"ready"`
		Players   int    `json:"players"`
		Items     int    `json:"items"`
		Mails     int    `json:"mails"`
		Audits    int    `json:"audits"`
		Tunables  int    `json:"tunables"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rr.Body.String())
	}
	if !out.Ready {
		t.Fatalf("ready=false, want true; body=%s", rr.Body.String())
	}
	if out.Admin != "/admin/" || out.AdminUser != "admin" || out.Host != "127.0.0.1:9500" {
		t.Fatalf("unexpected identity fields: %+v", out)
	}
	if out.Players != 0 || out.Items != 0 || out.Mails != 0 || out.Audits != 0 {
		t.Fatalf("新库计数应为 0，实得 %+v", out)
	}
	if out.Tunables == 0 {
		t.Fatalf("可调参数条数应大于 0，实得 %d", out.Tunables)
	}
}

func TestOverviewWithoutAdminDB(t *testing.T) {
	a := &app{cfg: &config.Config{Admin: config.Admin{Username: "admin"}}}
	rr := httptest.NewRecorder()
	a.handleOverview(rr, httptest.NewRequest(http.MethodGet, "/api/overview", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), `"ready":false`) {
		t.Fatalf("未就绪时应报 ready=false: %s", rr.Body.String())
	}
}
