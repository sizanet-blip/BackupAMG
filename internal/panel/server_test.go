package panel

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPanelRequiresThisComputerAndASingleUseToken(t *testing.T) {
	s, ts := testPanel(t)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	req.Host = "8.8.8.8"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMisdirectedRequest {
		t.Fatalf("host ajeno: %d", res.StatusCode)
	}

	token := s.IssueToken(`ALBERTOM\Alberto`, RoleUser)
	cookie := redeem(t, ts, token)
	again := redeemStatus(t, ts, token)
	if again != http.StatusUnauthorized {
		t.Fatalf("el token se pudo usar dos veces: %d", again)
	}

	home, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	home.AddCookie(cookie)
	page, err := http.DefaultClient.Do(home)
	if err != nil {
		t.Fatal(err)
	}
	defer page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("inicio: %d", page.StatusCode)
	}

	sessionReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/session", nil)
	sessionReq.AddCookie(cookie)
	sessionRes, err := http.DefaultClient.Do(sessionReq)
	if err != nil {
		t.Fatal(err)
	}
	defer sessionRes.Body.Close()
	body, _ := io.ReadAll(sessionRes.Body)
	if strings.Contains(strings.ToLower(string(body)), "password") || strings.Contains(string(body), "dpapi") {
		t.Fatalf("la sesión no puede incluir la contraseña: %s", body)
	}
	if !strings.Contains(string(body), `"role":"user"`) {
		t.Fatalf("sesión: %s", body)
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	s, ts := testPanel(t)
	now := time.Now()
	s.setNow(func() time.Time { return now })
	token := s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin)
	s.setNow(func() time.Time { return now.Add(61 * time.Second) })
	if status := redeemStatus(t, ts, token); status != http.StatusUnauthorized {
		t.Fatalf("caducado: %d", status)
	}
}

func TestMutationsNeedOriginCSRFAndAdminRole(t *testing.T) {
	s, ts := testPanel(t)
	userCookie := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	csrf := csrfOf(t, ts, userCookie)

	without := post(t, ts, "/api/v1/logout", userCookie, csrf, "")
	if without != http.StatusForbidden {
		t.Fatalf("sin origen: %d", without)
	}
	wrong := post(t, ts, "/api/v1/logout", userCookie, "no", "http://127.0.0.1:"+portOf(ts))
	if wrong != http.StatusForbidden {
		t.Fatalf("csrf malo: %d", wrong)
	}
	denied := post(t, ts, "/api/v1/admin/probe", userCookie, csrf, "http://127.0.0.1:"+portOf(ts))
	if denied != http.StatusForbidden {
		t.Fatalf("usuario normal: %d", denied)
	}

	adminCookie := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	adminCSRF := csrfOf(t, ts, adminCookie)
	allowed := post(t, ts, "/api/v1/admin/probe", adminCookie, adminCSRF, "http://localhost:"+portOf(ts))
	if allowed != http.StatusOK {
		t.Fatalf("administrador: %d", allowed)
	}
	ok := post(t, ts, "/api/v1/logout", adminCookie, adminCSRF, "http://127.0.0.1:"+portOf(ts))
	if ok != http.StatusOK {
		t.Fatalf("logout: %d", ok)
	}
}

func TestHomeActionsRespectRoleAndPublishStatus(t *testing.T) {
	s, ts := testPanel(t)
	var started string
	s.SetHooks(panelHooks(t, &started))
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	csrf := csrfOf(t, ts, user)

	statusRes := get(t, ts, "/api/v1/status", user)
	defer statusRes.Body.Close()
	body, _ := io.ReadAll(statusRes.Body)
	if statusRes.StatusCode != http.StatusOK || !strings.Contains(string(body), `"level":"green"`) {
		t.Fatalf("estado: %d %s", statusRes.StatusCode, body)
	}
	if strings.Contains(strings.ToLower(string(body)), "password") || strings.Contains(string(body), "dpapi") {
		t.Fatalf("el estado no puede incluir la contraseña: %s", body)
	}
	if post(t, ts, "/api/v1/verify", user, csrf, "http://127.0.0.1:"+portOf(ts)) != http.StatusForbidden {
		t.Fatal("un usuario normal no puede verificar")
	}
	if post(t, ts, "/api/v1/backup", user, csrf, "http://127.0.0.1:"+portOf(ts)) != http.StatusAccepted {
		t.Fatal("un usuario normal sí puede copiar")
	}
	if started != `ALBERTOM\Alberto` {
		t.Fatalf("la copia no llegó al agente: %q", started)
	}

	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	adminCSRF := csrfOf(t, ts, admin)
	if post(t, ts, "/api/v1/verify", admin, adminCSRF, "http://127.0.0.1:"+portOf(ts)) != http.StatusAccepted {
		t.Fatal("el administrador no pudo verificar")
	}
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	req.AddCookie(admin)
	stream, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	defer cancel()
	buf := make([]byte, 256)
	n, _ := stream.Body.Read(buf)
	if !strings.Contains(string(buf[:n]), `"level":"green"`) {
		t.Fatalf("eventos: %s", buf[:n])
	}
}

func TestOnlyAnAdminCanSaveConfig(t *testing.T) {
	s, ts := testPanel(t)
	var saved string
	s.SetHooks(Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		LoadConfig: func() (any, error) {
			return map[string]any{"sources": []string{`C:\Datos`}, "times": []string{"13:30"}}, nil
		},
		SaveConfig: func(raw []byte) ([]string, error) {
			saved = string(raw)
			if strings.Contains(saved, `"times":[]`) {
				return []string{"schedule.times: indica al menos una hora"}, nil
			}
			return nil, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	csrf := csrfOf(t, ts, user)
	if postBody(t, ts, "/api/v1/config", user, csrf, `{"times":["13:30"]}`) != http.StatusForbidden {
		t.Fatal("un usuario normal no puede guardar")
	}
	if saved != "" {
		t.Fatal("se intentó guardar sin permiso")
	}
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	adminCSRF := csrfOf(t, ts, admin)
	got := get(t, ts, "/api/v1/config", admin)
	defer got.Body.Close()
	body, _ := io.ReadAll(got.Body)
	if got.StatusCode != http.StatusOK || strings.Contains(string(body), "password") || !strings.Contains(string(body), `C:\\Datos`) {
		t.Fatalf("lectura: %d %s", got.StatusCode, body)
	}
	if postBody(t, ts, "/api/v1/config", admin, adminCSRF, `{"times":[]}`) != http.StatusBadRequest {
		t.Fatal("una hora vacía debería rechazarse")
	}
	if postBody(t, ts, "/api/v1/config", admin, adminCSRF, `{"times":["08:00"]}`) != http.StatusOK {
		t.Fatal("el administrador no pudo guardar")
	}
}

func TestDestinationAndPasswordAreAdminOnly(t *testing.T) {
	s, ts := testPanel(t)
	secret := "Secreto-que-no-puede-salir"
	var saved string
	s.SetHooks(Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		LoadDestination: func() (any, error) {
			return map[string]any{"path": `E:\Copias`, "kind": "local"}, nil
		},
		SaveDestination: func(raw []byte) ([]string, error) {
			saved = string(raw)
			if strings.Contains(saved, "relativa") {
				return []string{"repository.path: la ruta debe ser absoluta"}, nil
			}
			return nil, nil
		},
		TestDestination: func(raw []byte) (any, error) {
			return map[string]any{"ok": true, "message": "Se puede usar."}, nil
		},
		ChangePassword: func(raw []byte) ([]string, error) {
			if strings.Contains(string(raw), `"acknowledged":false`) {
				return []string{"Marca que has guardado la contraseña en un lugar seguro."}, nil
			}
			return nil, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	csrf := csrfOf(t, ts, user)
	res := get(t, ts, "/api/v1/destination", user)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatal("un usuario normal no puede ver el destino")
	}
	if postBody(t, ts, "/api/v1/destination", user, csrf, `{"path":"E:\\Copias"}`) != http.StatusForbidden {
		t.Fatal("un usuario normal no puede cambiar el destino")
	}
	if postBody(t, ts, "/api/v1/password", user, csrf, `{"password":"`+secret+`","confirm":"`+secret+`","acknowledged":true}`) != http.StatusForbidden {
		t.Fatal("un usuario normal no puede cambiar la contraseña")
	}
	if saved != "" {
		t.Fatal("se intentó guardar el destino sin permiso")
	}

	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	adminCSRF := csrfOf(t, ts, admin)
	got := get(t, ts, "/api/v1/destination", admin)
	body, _ := io.ReadAll(got.Body)
	got.Body.Close()
	if got.StatusCode != http.StatusOK || strings.Contains(string(body), "password") || !strings.Contains(string(body), `E:\\Copias`) {
		t.Fatalf("destino: %d %s", got.StatusCode, body)
	}
	if postBody(t, ts, "/api/v1/destination", admin, adminCSRF, `{"path":"relativa"}`) != http.StatusBadRequest {
		t.Fatal("una ruta relativa debería rechazarse")
	}
	if postBody(t, ts, "/api/v1/destination/test", admin, adminCSRF, `{"path":"E:\\Copias"}`) != http.StatusOK {
		t.Fatal("no se pudo probar el destino")
	}
	rejected := postRaw(t, ts, "/api/v1/password", admin, adminCSRF, `{"password":"`+secret+`","confirm":"otra","acknowledged":false}`)
	defer rejected.Body.Close()
	rejectedBody, _ := io.ReadAll(rejected.Body)
	if rejected.StatusCode != http.StatusBadRequest || strings.Contains(string(rejectedBody), secret) {
		t.Fatalf("contraseña rechazada: %d %s", rejected.StatusCode, rejectedBody)
	}
}

func TestSetupStaysWithTheAdmin(t *testing.T) {
	s, ts := testPanel(t)
	secret := "Secreto-que-no-puede-salir"
	s.SetHooks(Hooks{
		SetupStatus: func() (any, error) {
			return map[string]any{"needed": true, "hostname": "PC-NUEVO"}, nil
		},
		CompleteSetup: func(raw []byte) ([]string, error) {
			if strings.Contains(string(raw), `"acknowledged":false`) {
				return []string{"Marca que has guardado la contraseña en un lugar seguro."}, nil
			}
			return nil, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	csrf := csrfOf(t, ts, user)
	got := get(t, ts, "/api/v1/setup", user)
	body, _ := io.ReadAll(got.Body)
	got.Body.Close()
	if got.StatusCode != http.StatusOK || !strings.Contains(string(body), `"needed":true`) || strings.Contains(string(body), "password") {
		t.Fatalf("estado: %d %s", got.StatusCode, body)
	}
	if postBody(t, ts, "/api/v1/setup", user, csrf, `{"acknowledged":true}`) != http.StatusForbidden {
		t.Fatal("un usuario normal no puede terminar el asistente")
	}
	admin := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin))
	adminCSRF := csrfOf(t, ts, admin)
	rejected := postRaw(t, ts, "/api/v1/setup", admin, adminCSRF, `{"password":"`+secret+`","confirm":"`+secret+`","acknowledged":false}`)
	defer rejected.Body.Close()
	rejectedBody, _ := io.ReadAll(rejected.Body)
	if rejected.StatusCode != http.StatusBadRequest || strings.Contains(string(rejectedBody), secret) {
		t.Fatalf("asistente: %d %s", rejected.StatusCode, rejectedBody)
	}
}

func TestHistoryIsVisibleToANormalUser(t *testing.T) {
	s, ts := testPanel(t)
	s.SetHooks(Hooks{
		History: func(status, from, to string) (any, error) {
			if status == "rota" {
				return nil, errHistory
			}
			return []map[string]string{{"id": "2026-11-13_23-06-29.103", "status": "success"}}, nil
		},
		HistoryItem: func(id string) (any, error) {
			if id != "2026-11-13_23-06-29.103" {
				return nil, errHistory
			}
			return map[string]string{"status": "success", "message": "Copia completada"}, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	got := get(t, ts, "/api/v1/history", user)
	body, _ := io.ReadAll(got.Body)
	got.Body.Close()
	if got.StatusCode != http.StatusOK || strings.Contains(string(body), "password") || !strings.Contains(string(body), "success") {
		t.Fatalf("historial: %d %s", got.StatusCode, body)
	}
	bad := get(t, ts, "/api/v1/history?status=rota", user)
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatal("un filtro inválido debería rechazarse")
	}
	item := get(t, ts, "/api/v1/history/2026-11-13_23-06-29.103", user)
	itemBody, _ := io.ReadAll(item.Body)
	item.Body.Close()
	if item.StatusCode != http.StatusOK || strings.Contains(string(itemBody), "password") {
		t.Fatalf("detalle: %d %s", item.StatusCode, itemBody)
	}
	token := s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin)
	res := login(t, ts, token+"&next=historial")
	defer res.Body.Close()
	if res.Header.Get("Location") != "/?pantalla=historial" {
		t.Fatalf("destino del historial: %s", res.Header.Get("Location"))
	}
	other := login(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleAdmin)+"&next=http://otro")
	defer other.Body.Close()
	if other.Header.Get("Location") != "/" {
		t.Fatalf("destino ajeno: %s", other.Header.Get("Location"))
	}
}

func TestANormalUserCannotRestore(t *testing.T) {
	s, ts := testPanel(t)
	s.SetHooks(Hooks{
		Snapshots: func() (any, error) { return []any{}, nil },
		StartRestore: func(raw []byte) ([]string, error) {
			t.Fatal("un usuario normal no puede llegar a restaurar")
			return nil, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	csrf := csrfOf(t, ts, user)
	res := get(t, ts, "/api/v1/snapshots", user)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatal("un usuario normal no puede ver las copias")
	}
	if postBody(t, ts, "/api/v1/restore", user, csrf, `{"snapshot":"abcdef12","paths":["C:\\Datos"],"mode":"folder","target":"C:\\Temporal"}`) != http.StatusForbidden {
		t.Fatal("un usuario normal no puede recuperar")
	}
	page := get(t, ts, "/restaurar.js", user)
	defer page.Body.Close()
	body, _ := io.ReadAll(page.Body)
	if page.StatusCode != http.StatusOK || !strings.Contains(string(body), "pintarRestaurar") {
		t.Fatalf("la pantalla de restaurar no se sirve: %d", page.StatusCode)
	}
}

func TestANormalUserCannotReadDiagnostics(t *testing.T) {
	s, ts := testPanel(t)
	s.SetHooks(Hooks{
		Diagnostics: func() (any, error) {
			t.Fatal("un usuario normal no puede ver los ajustes")
			return nil, nil
		},
		DiagnosticBundle: func() ([]byte, error) {
			t.Fatal("un usuario normal no puede descargar el paquete")
			return nil, nil
		},
	})
	user := redeem(t, ts, s.IssueToken(`ALBERTOM\Alberto`, RoleUser))
	res := get(t, ts, "/api/v1/diagnostics", user)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatal("un usuario normal no puede ver el diagnóstico")
	}
	pack := get(t, ts, "/api/v1/diagnostics/bundle", user)
	pack.Body.Close()
	if pack.StatusCode != http.StatusForbidden {
		t.Fatal("un usuario normal no puede descargar el paquete")
	}
	script := get(t, ts, "/ajustes.js", user)
	defer script.Body.Close()
	body, _ := io.ReadAll(script.Body)
	if script.StatusCode != http.StatusOK || !strings.Contains(string(body), "pintarAjustes") {
		t.Fatalf("la pantalla de ajustes no se sirve: %d", script.StatusCode)
	}
}

var errHistory = errString("no")

type errString string

func (e errString) Error() string { return string(e) }

func postBody(t *testing.T, ts *httptest.Server, path string, cookie *http.Cookie, csrf, body string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	req.Header.Set("Origin", "http://127.0.0.1:"+portOf(ts))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func postRaw(t *testing.T, ts *httptest.Server, path string, cookie *http.Cookie, csrf, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	req.Header.Set("Origin", "http://127.0.0.1:"+portOf(ts))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func panelHooks(t *testing.T, started *string) Hooks {
	t.Helper()
	return Hooks{
		Status: func() any { return map[string]string{"level": "green"} },
		Backup: func(user string) error { *started = user; return nil },
		Verify: func(string) error { return nil },
		Cancel: func(string) error { return nil },
	}
}

func get(t *testing.T, ts *httptest.Server, path string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestListenAddressIsLoopbackOnly(t *testing.T) {
	if got := ListenAddr(47800); got != "127.0.0.1:47800" {
		t.Fatal(got)
	}
}

func testPanel(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s := New(Options{Port: 1})
	ts := httptest.NewServer(s.routes())
	s.port = ts.Listener.Addr().(*net.TCPAddr).Port
	t.Cleanup(ts.Close)
	return s, ts
}

func redeem(t *testing.T, ts *httptest.Server, token string) *http.Cookie {
	t.Helper()
	res := login(t, ts, token)
	defer res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %d", res.StatusCode)
	}
	for _, cookie := range res.Cookies() {
		if cookie.Name == cookieName && cookie.HttpOnly && cookie.SameSite == http.SameSiteStrictMode {
			return cookie
		}
	}
	t.Fatal("no llegó la cookie de sesión")
	return nil
}

func redeemStatus(t *testing.T, ts *httptest.Server, token string) int {
	t.Helper()
	res := login(t, ts, token)
	defer res.Body.Close()
	return res.StatusCode
}

func login(t *testing.T, ts *httptest.Server, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/login?token="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func csrfOf(t *testing.T, ts *httptest.Server, cookie *http.Cookie) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/session", nil)
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	const key = `"csrf":"`
	i := strings.Index(string(body), key)
	if i < 0 {
		t.Fatalf("sesión: %s", body)
	}
	rest := string(body)[i+len(key):]
	return rest[:strings.Index(rest, `"`)]
}

func post(t *testing.T, ts *httptest.Server, path string, cookie *http.Cookie, csrf, origin string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
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
	res.Body.Close()
	return res.StatusCode
}

func portOf(ts *httptest.Server) string {
	return strings.TrimPrefix(ts.Listener.Addr().String(), "127.0.0.1:")
}
