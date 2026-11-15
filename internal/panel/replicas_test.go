package panel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func send(t *testing.T, ts *httptest.Server, method, path string, cookie *http.Cookie, csrf, origin, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(data)
}

func replicaPanel(t *testing.T, calls *[]string) (*Server, *httptest.Server) {
	s, ts := testPanel(t)
	s.SetHooks(Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		Replicas: func() (any, error) {
			*calls = append(*calls, "list")
			return map[string]any{"replicas": []any{}}, nil
		},
		AddReplica: func(raw []byte) ([]string, error) {
			*calls = append(*calls, "add")
			if strings.Contains(string(raw), "dentro") {
				return []string{"La copia adicional no puede estar dentro de la copia principal."}, nil
			}
			return nil, nil
		},
		UpdateReplica: func(id string, raw []byte) ([]string, error) {
			*calls = append(*calls, "update:"+id)
			return nil, nil
		},
		RemoveReplica: func(id string) error { *calls = append(*calls, "remove:"+id); return nil },
		TestReplica: func(raw []byte) (any, error) {
			*calls = append(*calls, "test")
			return map[string]any{"ok": true}, nil
		},
		RunReplica: func(id string) error {
			*calls = append(*calls, "run:"+id)
			if id == "ocupada" {
				return errString("Ya hay una tarea en curso.")
			}
			return nil
		},
	})
	return s, ts
}

func TestReplicasAreAdminOnlyWithOriginAndCSRF(t *testing.T) {
	var calls []string
	s, ts := replicaPanel(t, &calls)
	origin := "http://127.0.0.1:" + portOf(ts)
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	userCSRF := csrfOf(t, ts, user)

	requests := []struct{ method, path, body string }{
		{"GET", "/api/v1/replicas", ""},
		{"POST", "/api/v1/replicas", `{"name":"x","path":"F:\\x"}`},
		{"POST", "/api/v1/replicas/test", `{"path":"F:\\x"}`},
		{"PUT", "/api/v1/replicas/usb", `{"enabled":false}`},
		{"DELETE", "/api/v1/replicas/usb", ""},
		{"POST", "/api/v1/replicas/usb/run", `{}`},
	}
	for _, rq := range requests {
		code, _ := send(t, ts, rq.method, rq.path, user, userCSRF, origin, rq.body)
		if code != http.StatusForbidden {
			t.Fatalf("un usuario normal no puede %s %s (%d)", rq.method, rq.path, code)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("se ejecutó algo sin permiso: %v", calls)
	}

	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	csrf := csrfOf(t, ts, admin)
	// Sin CSRF o con otro origen, ni siquiera un administrador puede cambiar nada.
	for _, rq := range requests[1:] {
		if code, _ := send(t, ts, rq.method, rq.path, admin, "", origin, rq.body); code != http.StatusForbidden {
			t.Fatalf("sin CSRF %s %s: %d", rq.method, rq.path, code)
		}
		if code, _ := send(t, ts, rq.method, rq.path, admin, csrf, "http://evil.example", rq.body); code != http.StatusForbidden {
			t.Fatalf("con otro origen %s %s: %d", rq.method, rq.path, code)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("se ejecutó algo sin CSRF o con otro origen: %v", calls)
	}

	for _, rq := range requests {
		want := http.StatusOK
		if rq.path == "/api/v1/replicas/usb/run" {
			want = http.StatusAccepted
		}
		if code, body := send(t, ts, rq.method, rq.path, admin, csrf, origin, rq.body); code != want {
			t.Fatalf("administrador %s %s: %d %s", rq.method, rq.path, code, body)
		}
	}
	want := "list,add,test,update:usb,remove:usb,run:usb"
	if got := strings.Join(calls, ","); got != want {
		t.Fatalf("llamadas %s, se esperaba %s", got, want)
	}
}

func TestReplicaErrorsReachThePanelWithACause(t *testing.T) {
	var calls []string
	s, ts := replicaPanel(t, &calls)
	origin := "http://127.0.0.1:" + portOf(ts)
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	csrf := csrfOf(t, ts, admin)

	code, body := send(t, ts, "POST", "/api/v1/replicas", admin, csrf, origin, `{"path":"dentro"}`)
	if code != http.StatusBadRequest || !strings.Contains(body, "dentro de la copia principal") {
		t.Fatalf("un problema de validación debe llegar como 400 con su causa: %d %s", code, body)
	}
	code, body = send(t, ts, "POST", "/api/v1/replicas/ocupada/run", admin, csrf, origin, `{}`)
	if code != http.StatusConflict || !strings.Contains(body, "en curso") {
		t.Fatalf("una tarea en curso debe llegar como 409: %d %s", code, body)
	}
}

func TestReplicasWithoutHooksAreNotReady(t *testing.T) {
	s, ts := testPanel(t)
	s.SetHooks(Hooks{Status: func() any { return map[string]string{"level": "green"} }})
	origin := "http://127.0.0.1:" + portOf(ts)
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	csrf := csrfOf(t, ts, admin)
	for _, rq := range []struct{ method, path string }{
		{"GET", "/api/v1/replicas"}, {"POST", "/api/v1/replicas"}, {"PUT", "/api/v1/replicas/a"},
		{"DELETE", "/api/v1/replicas/a"}, {"POST", "/api/v1/replicas/a/run"}, {"POST", "/api/v1/replicas/test"},
	} {
		if code, _ := send(t, ts, rq.method, rq.path, admin, csrf, origin, `{}`); code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s sin agente listo: %d", rq.method, rq.path, code)
		}
	}
}

// Recuperar desde una copia adicional: el identificador viaja en ?replica= y por defecto es la principal.
func TestSnapshotsCanBeReadFromAReplica(t *testing.T) {
	s, ts := testPanel(t)
	var seen []string
	s.SetHooks(Hooks{
		Status:    func() any { return map[string]string{"level": "green"} },
		Snapshots: func(replica string) (any, error) { seen = append(seen, "list:"+replica); return []any{}, nil },
		SnapshotFiles: func(id, path, replica string) (any, error) {
			seen = append(seen, "files:"+id+":"+replica)
			return []any{}, nil
		},
		SnapshotFind: func(id, pattern, replica string) (any, error) {
			seen = append(seen, "find:"+id+":"+replica)
			return []any{}, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	for _, path := range []string{"/api/v1/snapshots?replica=usb", "/api/v1/snapshots/abcdef12/files?replica=usb", "/api/v1/snapshots/abcdef12/find?q=x&replica=usb"} {
		res := get(t, ts, path, user)
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("un usuario normal no puede leer %s (%d)", path, res.StatusCode)
		}
	}
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	for _, path := range []string{"/api/v1/snapshots", "/api/v1/snapshots?replica=usb", "/api/v1/snapshots/abcdef12/files?replica=usb", "/api/v1/snapshots/abcdef12/find?q=x&replica=usb", "/api/v1/snapshots/abcdef12/files"} {
		res := get(t, ts, path, admin)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
	}
	want := "list:,list:usb,files:abcdef12:usb,find:abcdef12:usb,files:abcdef12:"
	if got := strings.Join(seen, ","); got != want {
		t.Fatalf("llamadas %s, se esperaba %s", got, want)
	}
}

// Un aviso que acompaña a un cambio correcto llega como «notice», y no oculta el «ok».
func TestPasswordChangeCanReturnANotice(t *testing.T) {
	s, ts := testPanel(t)
	s.SetHooks(Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		ChangePassword: func(raw []byte) ([]string, error) {
			if strings.Contains(string(raw), "bloqueada") {
				return []string{"Conecta «Disco externo» o quítala antes de cambiar la contraseña."}, nil
			}
			return nil, &Notice{Message: "La antigua todavía abre «Disco externo»."}
		},
	})
	origin := "http://127.0.0.1:" + portOf(ts)
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	csrf := csrfOf(t, ts, admin)
	code, body := send(t, ts, "POST", "/api/v1/password", admin, csrf, origin, `{"password":"Zq9-pw","confirm":"Zq9-pw","acknowledged":true}`)
	if code != http.StatusOK || !strings.Contains(body, `"ok":true`) || !strings.Contains(body, "todavía abre") {
		t.Fatalf("%d %s", code, body)
	}
	code, body = send(t, ts, "POST", "/api/v1/password", admin, csrf, origin, `{"password":"bloqueada","confirm":"bloqueada","acknowledged":true}`)
	if code != http.StatusBadRequest || !strings.Contains(body, "Conecta «Disco externo»") || strings.Contains(body, "bloqueada") {
		t.Fatalf("%d %s", code, body)
	}
}

// La conexión con la plataforma solo la gestiona un administrador, con Origin y CSRF.
func TestCloudRoutesAreAdminOnly(t *testing.T) {
	s, ts := testPanel(t)
	var calls []string
	s.SetHooks(Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		Cloud:  func() (any, error) { calls = append(calls, "view"); return map[string]any{"state": "connected"}, nil },
		CloudEnroll: func(raw []byte) ([]string, error) {
			calls = append(calls, "enroll")
			if strings.Contains(string(raw), "mala") {
				return []string{"La clave de licencia no es válida."}, nil
			}
			return nil, nil
		},
		CloudSettings: func(raw []byte) ([]string, error) { calls = append(calls, "settings"); return nil, nil },
		CloudSync:     func() error { calls = append(calls, "sync"); return nil },
	})
	origin := "http://127.0.0.1:" + portOf(ts)
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	userCSRF := csrfOf(t, ts, user)
	requests := []struct{ method, path, body string }{
		{"GET", "/api/v1/cloud", ""},
		{"POST", "/api/v1/cloud/enroll", `{"credential":"BKA-1"}`},
		{"PUT", "/api/v1/cloud/settings", `{"server_url":"https://x"}`},
		{"POST", "/api/v1/cloud/sync", `{}`},
	}
	for _, rq := range requests {
		if code, _ := send(t, ts, rq.method, rq.path, user, userCSRF, origin, rq.body); code != http.StatusForbidden {
			t.Fatalf("un usuario normal no puede %s %s (%d)", rq.method, rq.path, code)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("se ejecutó algo sin permiso: %v", calls)
	}
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	csrf := csrfOf(t, ts, admin)
	for _, rq := range requests[1:] {
		if code, _ := send(t, ts, rq.method, rq.path, admin, "", origin, rq.body); code != http.StatusForbidden {
			t.Fatalf("sin CSRF %s %s: %d", rq.method, rq.path, code)
		}
		if code, _ := send(t, ts, rq.method, rq.path, admin, csrf, "http://evil.example", rq.body); code != http.StatusForbidden {
			t.Fatalf("con otro origen %s %s: %d", rq.method, rq.path, code)
		}
	}
	for _, rq := range requests {
		want := http.StatusOK
		if rq.path == "/api/v1/cloud/sync" {
			want = http.StatusAccepted
		}
		if code, body := send(t, ts, rq.method, rq.path, admin, csrf, origin, rq.body); code != want {
			t.Fatalf("administrador %s %s: %d %s", rq.method, rq.path, code, body)
		}
	}
	if got := strings.Join(calls, ","); got != "view,enroll,settings,sync" {
		t.Fatalf("llamadas %s", got)
	}
	code, body := send(t, ts, "POST", "/api/v1/cloud/enroll", admin, csrf, origin, `{"credential":"mala"}`)
	if code != http.StatusBadRequest || !strings.Contains(body, "no es válida") {
		t.Fatalf("una clave mala debe llegar como 400 con su causa: %d %s", code, body)
	}
}
