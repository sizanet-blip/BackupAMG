//go:build windows

package panel

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"backup-agent/internal/status"
)

func TestPipeIssuesAOneUseLogin(t *testing.T) {
	PipeName = `\\.\pipe\BackupAgent-test-login`
	t.Cleanup(func() { PipeName = `\\.\pipe\BackupAgent` })
	s := New(Options{Port: 47921})
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(200 * time.Millisecond)
	}()
	go func() { _ = s.Serve(ctx) }()

	var url string
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		url, _, _, err = RequestToken()
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:47921/login?token=") {
		t.Fatalf("url: %s", url)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %d", res.StatusCode)
	}
	res2, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("segundo uso: %d", res2.StatusCode)
	}
}

func TestPipeReportsStatusAndStartsBackup(t *testing.T) {
	PipeName = `\\.\pipe\BackupAgent-test-status`
	t.Cleanup(func() { PipeName = `\\.\pipe\BackupAgent` })
	s := New(Options{Port: 47922})
	var started string
	s.SetHooks(Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		Backup: func(user string) error { started = user; return nil },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(200 * time.Millisecond)
	}()
	go func() { _ = s.Serve(ctx) }()

	var view status.View
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		view, err = RequestStatus()
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if view.Level != "green" {
		t.Fatalf("nivel %s", view.Level)
	}
	if err := RequestBackup(); err != nil {
		t.Fatal(err)
	}
	if started == "" {
		t.Fatal("la copia no llegó al agente")
	}
}
