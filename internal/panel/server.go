// Package panel sirve la interfaz local del agente.
// Escucha solo en 127.0.0.1 y exige un token de un solo uso para entrar.
package panel

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFS embed.FS

// PipeName es el canal por el que la bandeja pide tokens. No usa HTTP.
var PipeName = `\\.\pipe\BackupAgent`

const (
	tokenTTL   = 60 * time.Second
	sessionTTL = 8 * time.Hour
	cookieName = "ba_session"
)

var errHookMissing = errors.New("hook")

// Role es el permiso de quien abre el panel.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// Options configura el panel.
type Options struct {
	Port int
	Log  *slog.Logger
	Now  func() time.Time
}

// Server es el panel local.
type Server struct {
	port int
	log  *slog.Logger
	now  func() time.Time

	mu       sync.Mutex
	tokens   map[string]ticket
	sessions map[string]session
	hooks    Hooks
	subs     map[chan []byte]struct{}
}

// Hooks conecta el panel con las copias y la configuración del agente.
type Hooks struct {
	Status          func() any
	Backup          func(user string) error
	Verify          func(user string) error
	Cancel          func(user string) error
	LoadConfig      func() (any, error)
	SaveConfig      func(raw []byte) (problems []string, err error)
	LoadDestination func() (any, error)
	SaveDestination func(raw []byte) (problems []string, err error)
	TestDestination func(raw []byte) (any, error)
	ChangePassword  func(raw []byte) (problems []string, err error)
	SetupStatus     func() (any, error)
	CompleteSetup   func(raw []byte) (problems []string, err error)
	History         func(status, from, to string) (any, error)
	HistoryItem     func(id string) (any, error)
	// El último argumento es el identificador de la copia adicional; vacío es la copia principal.
	Snapshots        func(replica string) (any, error)
	SnapshotFiles    func(id, path, replica string) (any, error)
	SnapshotFind     func(id, pattern, replica string) (any, error)
	RestoreTarget    func() (any, error)
	StartRestore     func(raw []byte) (problems []string, err error)
	Diagnostics      func() (any, error)
	DiagnosticLog    func(level string) (any, error)
	DiagnosticBundle func() ([]byte, error)
	Folders          func(path string) (any, error)
	Shortcuts        func() any
	Estimate         func(paths []string) (bytes int64, complete bool, err error)
	// Copias adicionales (réplicas).
	Replicas      func() (any, error)
	AddReplica    func(raw []byte) (problems []string, err error)
	UpdateReplica func(id string, raw []byte) (problems []string, err error)
	RemoveReplica func(id string) error
	TestReplica   func(raw []byte) (any, error)
	RunReplica    func(id string) error
	// Conexión con la plataforma web.
	Cloud         func() (any, error)
	CloudEnroll   func(raw []byte) (problems []string, err error)
	CloudSettings func(raw []byte) (problems []string, err error)
	CloudSync     func() error
}

type ticket struct {
	User    string
	Role    Role
	Expires time.Time
}

type session struct {
	User    string
	Role    Role
	CSRF    string
	Expires time.Time
}

// New prepara el panel. El puerto 0 usa el predeterminado.
func New(opts Options) *Server {
	port := opts.Port
	if port == 0 {
		port = 47800
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		port:     port,
		log:      log,
		now:      now,
		tokens:   map[string]ticket{},
		sessions: map[string]session{},
		subs:     map[chan []byte]struct{}{},
	}
}

// SetHooks instala las acciones del agente. Se puede llamar antes de Serve.
func (s *Server) SetHooks(h Hooks) {
	s.mu.Lock()
	s.hooks = h
	s.mu.Unlock()
}

// Publish avisa a las pantallas abiertas de que el estado ha cambiado.
func (s *Server) Publish() {
	s.broadcast(s.snapshotJSON())
}

// Port es el puerto configurado.
func (s *Server) Port() int { return s.port }

// Serve escucha en 127.0.0.1 hasta que ctx se cancela.
// También abre el named pipe de la bandeja.
func (s *Server) Serve(ctx context.Context) error {
	go s.servePipe(ctx)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return fmt.Errorf("el puerto %d del panel está ocupado o no se puede abrir: %w", s.port, err)
	}
	srv := &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shut, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
		return nil
	case err := <-errCh:
		return err
	}
}

// IssueToken crea un token de un solo uso para user.
func (s *Server) IssueToken(user string, role Role) string {
	raw := randomToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropExpiredLocked()
	s.tokens[raw] = ticket{User: user, Role: role, Expires: s.now().Add(tokenTTL)}
	return raw
}

// LoginURL es la dirección que canjea el token. Caduca con él.
func (s *Server) LoginURL(token string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/login?token=%s", s.port, token)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", s.handleLogin)
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /textos.js", s.handleAsset)
	mux.HandleFunc("GET /app.js", s.handleAsset)
	mux.HandleFunc("GET /config.js", s.handleAsset)
	mux.HandleFunc("GET /destino.js", s.handleAsset)
	mux.HandleFunc("GET /copias.js", s.handleAsset)
	mux.HandleFunc("GET /asistente.js", s.handleAsset)
	mux.HandleFunc("GET /historial.js", s.handleAsset)
	mux.HandleFunc("GET /restaurar.js", s.handleAsset)
	mux.HandleFunc("GET /ajustes.js", s.handleAsset)
	mux.HandleFunc("GET /estilo.css", s.handleAsset)
	mux.HandleFunc("GET /fuentes/{nombre}", s.handleFont)
	mux.HandleFunc("GET /favicon.ico", s.handlePublicAsset)
	mux.HandleFunc("GET /marca.svg", s.handlePublicAsset)
	mux.HandleFunc("GET /marca-completa.svg", s.handlePublicAsset)
	mux.HandleFunc("GET /marca-barra.svg", s.handlePublicAsset)
	mux.HandleFunc("GET /api/v1/session", s.handleSession)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/events", s.handleEvents)
	mux.Handle("POST /api/v1/logout", s.mutating(http.HandlerFunc(s.handleLogout)))
	mux.Handle("POST /api/v1/backup", s.mutating(http.HandlerFunc(s.handleBackup)))
	mux.Handle("POST /api/v1/verify", s.admin(http.HandlerFunc(s.handleVerify)))
	mux.Handle("POST /api/v1/cancel", s.mutating(http.HandlerFunc(s.handleCancel)))
	mux.Handle("GET /api/v1/config", s.adminRead(http.HandlerFunc(s.handleLoadConfig)))
	mux.Handle("POST /api/v1/config", s.admin(http.HandlerFunc(s.handleSaveConfig)))
	mux.Handle("GET /api/v1/destination", s.adminRead(http.HandlerFunc(s.handleLoadDestination)))
	mux.Handle("POST /api/v1/destination", s.admin(http.HandlerFunc(s.handleSaveDestination)))
	mux.Handle("POST /api/v1/destination/test", s.admin(http.HandlerFunc(s.handleTestDestination)))
	mux.Handle("POST /api/v1/password", s.admin(http.HandlerFunc(s.handlePassword)))
	mux.Handle("GET /api/v1/cloud", s.adminRead(http.HandlerFunc(s.handleCloud)))
	mux.Handle("POST /api/v1/cloud/enroll", s.admin(http.HandlerFunc(s.handleCloudEnroll)))
	mux.Handle("PUT /api/v1/cloud/settings", s.admin(http.HandlerFunc(s.handleCloudSettings)))
	mux.Handle("POST /api/v1/cloud/sync", s.admin(http.HandlerFunc(s.handleCloudSync)))
	mux.Handle("GET /api/v1/replicas", s.adminRead(http.HandlerFunc(s.handleReplicas)))
	mux.Handle("POST /api/v1/replicas", s.admin(http.HandlerFunc(s.handleAddReplica)))
	mux.Handle("POST /api/v1/replicas/test", s.admin(http.HandlerFunc(s.handleTestReplica)))
	mux.Handle("PUT /api/v1/replicas/{id}", s.admin(http.HandlerFunc(s.handleUpdateReplica)))
	mux.Handle("DELETE /api/v1/replicas/{id}", s.admin(http.HandlerFunc(s.handleRemoveReplica)))
	mux.Handle("POST /api/v1/replicas/{id}/run", s.admin(http.HandlerFunc(s.handleRunReplica)))
	mux.Handle("GET /api/v1/setup", s.withSession(http.HandlerFunc(s.handleSetupStatus)))
	mux.Handle("POST /api/v1/setup", s.admin(http.HandlerFunc(s.handleSetup)))
	mux.Handle("GET /api/v1/history", s.withSession(http.HandlerFunc(s.handleHistory)))
	mux.Handle("GET /api/v1/history/{id}", s.withSession(http.HandlerFunc(s.handleHistoryItem)))
	mux.Handle("GET /api/v1/snapshots", s.adminRead(http.HandlerFunc(s.handleSnapshots)))
	mux.Handle("GET /api/v1/snapshots/{id}/files", s.adminRead(http.HandlerFunc(s.handleSnapshotFiles)))
	mux.Handle("GET /api/v1/snapshots/{id}/find", s.adminRead(http.HandlerFunc(s.handleSnapshotFind)))
	mux.Handle("GET /api/v1/restore/target", s.adminRead(http.HandlerFunc(s.handleRestoreTarget)))
	mux.Handle("POST /api/v1/restore", s.admin(http.HandlerFunc(s.handleRestore)))
	mux.Handle("GET /api/v1/diagnostics", s.adminRead(http.HandlerFunc(s.handleDiagnostics)))
	mux.Handle("GET /api/v1/diagnostics/log", s.adminRead(http.HandlerFunc(s.handleDiagnosticLog)))
	mux.Handle("GET /api/v1/diagnostics/bundle", s.adminRead(http.HandlerFunc(s.handleDiagnosticBundle)))
	mux.Handle("GET /api/v1/folders", s.adminRead(http.HandlerFunc(s.handleFolders)))
	mux.Handle("GET /api/v1/shortcuts", s.adminRead(http.HandlerFunc(s.handleShortcuts)))
	mux.Handle("POST /api/v1/size", s.admin(http.HandlerFunc(s.handleSize)))
	mux.Handle("POST /api/v1/admin/probe", s.admin(http.HandlerFunc(s.handleAdminProbe)))
	return s.guardHost(mux)
}

func (s *Server) guardHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			http.Error(w, "Este panel solo se abre desde este equipo.", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) hostAllowed(host string) bool {
	host = strings.TrimSpace(host)
	return host == fmt.Sprintf("127.0.0.1:%d", s.port) || host == fmt.Sprintf("localhost:%d", s.port)
}

func (s *Server) originAllowed(origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	return origin == fmt.Sprintf("http://127.0.0.1:%d", s.port) || origin == fmt.Sprintf("http://localhost:%d", s.port)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("token")
	sess, ok := s.redeem(raw)
	if !ok {
		http.Error(w, "El enlace ha caducado o ya se ha usado. Vuelve a abrir el panel.", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sess,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	dest := "/"
	next := r.URL.Query().Get("next")
	if next == "historial" || next == "restaurar" || next == "ajustes" || next == "copiar" {
		dest = "/?pantalla=" + next
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFrom(r); !ok {
		http.Error(w, "Abre el panel con agent panel o desde el icono de la bandeja.", http.StatusUnauthorized)
		return
	}
	s.writeAsset(w, "web/index.html")
}

func (s *Server) handlePublicAsset(w http.ResponseWriter, r *http.Request) {
	s.writeAsset(w, "web/"+strings.TrimPrefix(r.URL.Path, "/"))
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFrom(r); !ok {
		http.Error(w, "Hace falta una sesión.", http.StatusUnauthorized)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	s.writeAsset(w, "web/"+name)
}

// fuentesPermitidas son las únicas fuentes que se sirven: Inter, subconjunto latino, licencia OFL.
var fuentesPermitidas = map[string]bool{
	"inter-latin-400-normal.woff2": true,
	"inter-latin-500-normal.woff2": true,
	"inter-latin-600-normal.woff2": true,
	"inter-latin-700-normal.woff2": true,
}

// handleFont sirve las fuentes incrustadas. Exige sesión como el resto del panel
// y solo admite los nombres de la lista cerrada.
func (s *Server) handleFont(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFrom(r); !ok {
		http.Error(w, "Hace falta una sesión.", http.StatusUnauthorized)
		return
	}
	name := r.PathValue("nombre")
	if !fuentesPermitidas[name] {
		http.Error(w, "No se encontró la página.", http.StatusNotFound)
		return
	}
	body, err := fs.ReadFile(webFS, "web/fuentes/"+name)
	if err != nil {
		http.Error(w, "No se encontró la página.", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "font/woff2")
	// El nombre del fichero lleva la versión de la fuente: se puede guardar mucho tiempo.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func (s *Server) writeAsset(w http.ResponseWriter, name string) {
	body, err := fs.ReadFile(webFS, name)
	if err != nil {
		http.Error(w, "No se encontró la página.", http.StatusNotFound)
		return
	}
	switch {
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(name, ".ico"):
		w.Header().Set("Content-Type", "image/x-icon")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Hace falta una sesión."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"user": sess.User,
		"role": string(sess.Role),
		"csrf": sess.CSRF,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionFrom(r)
	s.audit(sess.User, "cerrar sesión")
	s.dropSession(cookieValue(r))
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFrom(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Hace falta una sesión."})
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(s.snapshotJSON()))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionFrom(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Hace falta una sesión."})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Este servidor no puede enviar el avance.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	ch := s.subscribe()
	defer s.unsubscribe(ch)
	fmt.Fprintf(w, "data: %s\n\n", s.snapshotJSON())
	flusher.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case msg, open := <-ch:
			if !open {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	s.act(w, r, "iniciar copia", s.hookBackup)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	s.act(w, r, "iniciar verificación", s.hookVerify)
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.act(w, r, "cancelar la tarea", s.hookCancel)
}

func (s *Server) act(w http.ResponseWriter, r *http.Request, action string, fn func(string) error) {
	sess, _ := s.sessionFrom(r)
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	if err := fn(sess.User); err != nil {
		s.log.Warn("Acción del panel rechazada", "user", sess.User, "action", action, "error", err.Error())
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	s.audit(sess.User, action)
	s.Publish()
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) hookBackup(user string) error {
	s.mu.Lock()
	fn := s.hooks.Backup
	s.mu.Unlock()
	if fn == nil {
		return errors.New("El agente todavía no está listo.")
	}
	return fn(user)
}

func (s *Server) hookVerify(user string) error {
	s.mu.Lock()
	fn := s.hooks.Verify
	s.mu.Unlock()
	if fn == nil {
		return errors.New("El agente todavía no está listo.")
	}
	return fn(user)
}

func (s *Server) hookCancel(user string) error {
	s.mu.Lock()
	fn := s.hooks.Cancel
	s.mu.Unlock()
	if fn == nil {
		return errors.New("El agente todavía no está listo.")
	}
	return fn(user)
}

func (s *Server) snapshotJSON() []byte {
	s.mu.Lock()
	fn := s.hooks.Status
	s.mu.Unlock()
	if fn == nil {
		return []byte(`{}`)
	}
	raw, err := json.Marshal(fn())
	if err != nil {
		return []byte(`{"level":"red","problem":"No se pudo preparar el estado."}`)
	}
	return raw
}

func (s *Server) subscribe() chan []byte {
	ch := make(chan []byte, 8)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *Server) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
	s.mu.Unlock()
}

func (s *Server) broadcast(msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *Server) handleLoadConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.LoadConfig
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionFrom(r)
	s.mu.Lock()
	fn := s.hooks.SaveConfig
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer el formulario."})
		return
	}
	problems, err := fn(raw)
	if len(problems) > 0 {
		s.log.Warn("Acción del panel rechazada", "user", sess.User, "action", "guardar configuración")
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "La configuración no es válida.", "fields": problems})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	s.audit(sess.User, "guardar configuración")
	s.Publish()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLoadDestination(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.LoadDestination
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleSaveDestination(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.SaveDestination
	s.mu.Unlock()
	s.change(w, r, "guardar destino", 1<<20, fn)
}

func (s *Server) handleTestDestination(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.TestDestination
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer el formulario."})
		return
	}
	body, err := fn(raw)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": scrub(err.Error(), raw)})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) replicaHooks() Hooks {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hooks
}

func (s *Server) handleCloud(w http.ResponseWriter, r *http.Request) {
	fn := s.replicaHooks().Cloud
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleCloudEnroll(w http.ResponseWriter, r *http.Request) {
	s.change(w, r, "dar de alta en la plataforma", 16<<10, s.replicaHooks().CloudEnroll)
}

func (s *Server) handleCloudSettings(w http.ResponseWriter, r *http.Request) {
	s.change(w, r, "cambiar los ajustes de la plataforma", 16<<10, s.replicaHooks().CloudSettings)
}

func (s *Server) handleCloudSync(w http.ResponseWriter, r *http.Request) {
	fn := s.replicaHooks().CloudSync
	if fn == nil {
		s.act(w, r, "enviar a la plataforma", nil)
		return
	}
	s.act(w, r, "enviar a la plataforma", func(string) error { return fn() })
}

func (s *Server) handleReplicas(w http.ResponseWriter, r *http.Request) {
	fn := s.replicaHooks().Replicas
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleAddReplica(w http.ResponseWriter, r *http.Request) {
	s.change(w, r, "añadir copia adicional", 1<<20, s.replicaHooks().AddReplica)
}

func (s *Server) handleUpdateReplica(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fn := s.replicaHooks().UpdateReplica
	if fn == nil {
		s.change(w, r, "cambiar copia adicional", 1<<20, nil)
		return
	}
	s.change(w, r, "cambiar copia adicional", 1<<20, func(raw []byte) ([]string, error) { return fn(id, raw) })
}

func (s *Server) handleRemoveReplica(w http.ResponseWriter, r *http.Request) {
	fn := s.replicaHooks().RemoveReplica
	id := r.PathValue("id")
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	sess, _ := s.sessionFrom(r)
	if err := fn(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	s.audit(sess.User, "quitar copia adicional")
	s.Publish()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRunReplica(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fn := s.replicaHooks().RunReplica
	if fn == nil {
		s.act(w, r, "actualizar copia adicional", nil)
		return
	}
	s.act(w, r, "actualizar copia adicional", func(string) error { return fn(id) })
}

func (s *Server) handleTestReplica(w http.ResponseWriter, r *http.Request) {
	fn := s.replicaHooks().TestReplica
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer el formulario."})
		return
	}
	body, err := fn(raw)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.ChangePassword
	s.mu.Unlock()
	s.change(w, r, "cambiar contraseña", 16<<10, fn)
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.SetupStatus
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.CompleteSetup
	s.mu.Unlock()
	s.change(w, r, "asistente de configuración", 1<<20, fn)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.History
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn(r.URL.Query().Get("status"), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body, total, err := pageHistory(body, r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if total >= 0 {
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
	}
	writeJSON(w, http.StatusOK, body)
}

// pageHistory recorta la lista si se pide con limit (y, opcionalmente, offset).
// Sin limit devuelve la lista entera, igual que siempre. total es el número de ejecuciones
// que cumplen el filtro, o -1 si la respuesta no es una lista.
func pageHistory(body any, limitText, offsetText string) (any, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return body, -1, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return body, -1, nil
	}
	total := len(items)
	limitText = strings.TrimSpace(limitText)
	offsetText = strings.TrimSpace(offsetText)
	if limitText == "" && offsetText == "" {
		return body, total, nil
	}
	offset := 0
	if offsetText != "" {
		n, err := strconv.Atoi(offsetText)
		if err != nil || n < 0 {
			return nil, 0, errors.New("el desplazamiento no es válido")
		}
		offset = n
	}
	limit := total
	if limitText != "" {
		n, err := strconv.Atoi(limitText)
		if err != nil || n < 1 || n > 500 {
			return nil, 0, errors.New("el límite no es válido")
		}
		limit = n
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := items[offset:end]
	if page == nil {
		page = []json.RawMessage{}
	}
	return page, total, nil
}

func (s *Server) handleHistoryItem(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.HistoryItem
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "No se encontró esa ejecución."})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	s.readHook(w, func() (any, error) {
		s.mu.Lock()
		fn := s.hooks.Snapshots
		s.mu.Unlock()
		if fn == nil {
			return nil, errHookMissing
		}
		return fn(r.URL.Query().Get("replica"))
	})
}

func (s *Server) handleSnapshotFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dir := r.URL.Query().Get("path")
	s.readHook(w, func() (any, error) {
		s.mu.Lock()
		fn := s.hooks.SnapshotFiles
		s.mu.Unlock()
		if fn == nil {
			return nil, errHookMissing
		}
		return fn(id, dir, r.URL.Query().Get("replica"))
	})
}

func (s *Server) handleSnapshotFind(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pattern := r.URL.Query().Get("q")
	s.readHook(w, func() (any, error) {
		s.mu.Lock()
		fn := s.hooks.SnapshotFind
		s.mu.Unlock()
		if fn == nil {
			return nil, errHookMissing
		}
		return fn(id, pattern, r.URL.Query().Get("replica"))
	})
}

func (s *Server) handleRestoreTarget(w http.ResponseWriter, r *http.Request) {
	s.readHook(w, func() (any, error) {
		s.mu.Lock()
		fn := s.hooks.RestoreTarget
		s.mu.Unlock()
		if fn == nil {
			return nil, errHookMissing
		}
		return fn()
	})
}

func (s *Server) readHook(w http.ResponseWriter, fn func() (any, error)) {
	body, err := fn()
	if err == errHookMissing {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionFrom(r)
	s.mu.Lock()
	fn := s.hooks.StartRestore
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer el formulario."})
		return
	}
	problems, err := fn(raw)
	if len(problems) > 0 {
		s.log.Warn("Acción del panel rechazada", "user", sess.User, "action", "restaurar")
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "No se pudo recuperar.", "fields": problems})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	s.audit(sess.User, "restaurar")
	s.Publish()
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	s.readHook(w, func() (any, error) {
		s.mu.Lock()
		fn := s.hooks.Diagnostics
		s.mu.Unlock()
		if fn == nil {
			return nil, errHookMissing
		}
		return fn()
	})
}

func (s *Server) handleDiagnosticLog(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	s.readHook(w, func() (any, error) {
		s.mu.Lock()
		fn := s.hooks.DiagnosticLog
		s.mu.Unlock()
		if fn == nil {
			return nil, errHookMissing
		}
		return fn(level)
	})
}

func (s *Server) handleDiagnosticBundle(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionFrom(r)
	s.mu.Lock()
	fn := s.hooks.DiagnosticBundle
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.audit(sess.User, "paquete de diagnóstico")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="diagnostico.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

func (s *Server) change(w http.ResponseWriter, r *http.Request, action string, limit int64, fn func([]byte) ([]string, error)) {
	sess, _ := s.sessionFrom(r)
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, limit))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer el formulario."})
		return
	}
	problems, err := fn(raw)
	var notice *Notice
	if errors.As(err, &notice) {
		err = nil // salió bien, pero hay algo que contar
	}
	for i, problem := range problems {
		problems[i] = scrub(problem, raw)
	}
	if len(problems) > 0 {
		s.log.Warn("Acción del panel rechazada", "user", sess.User, "action", action)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "No se pudo guardar.", "fields": problems})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": scrub(err.Error(), raw)})
		return
	}
	s.audit(sess.User, action)
	s.Publish()
	if notice != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "notice": scrub(notice.Message, raw)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Notice es un aviso que acompaña a una acción que ha salido bien. Un gancho lo devuelve como
// error y el panel responde «ok» con el texto en «notice».
type Notice struct{ Message string }

func (n *Notice) Error() string { return n.Message }

func scrub(text string, raw []byte) string {
	var body struct {
		Password string `json:"password"`
		Confirm  string `json:"confirm"`
	}
	if json.Unmarshal(raw, &body) == nil {
		if body.Password != "" {
			text = strings.ReplaceAll(text, body.Password, "")
		}
		if body.Confirm != "" {
			text = strings.ReplaceAll(text, body.Confirm, "")
		}
	}
	return text
}

func (s *Server) handleFolders(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.Folders
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	body, err := fn(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleShortcuts(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.Shortcuts
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, fn())
}

func (s *Server) handleSize(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	fn := s.hooks.Estimate
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "El agente todavía no está listo."})
		return
	}
	var body struct {
		Sources []string `json:"sources"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "No se pudo leer la selección."})
		return
	}
	total, complete, err := fn(body.Sources)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bytes": total, "complete": complete})
}

func (s *Server) pipeResult(user string, role Role, op, screen string) any {
	switch op {
	case "token":
		token := s.IssueToken(user, role)
		s.log.Info("Token del panel emitido", "user", user, "role", role)
		return map[string]any{"ok": true, "url": s.LoginURL(token), "user": user, "role": role}
	case "show":
		return s.pipeShow(user, role, screen)
	case "status":
		s.mu.Lock()
		fn := s.hooks.Status
		s.mu.Unlock()
		if fn == nil {
			return map[string]any{"ok": false, "error": "El agente todavía no está listo."}
		}
		return map[string]any{"ok": true, "status": fn()}
	case "backup":
		if err := s.hookBackup(user); err != nil {
			s.log.Warn("Acción del panel rechazada", "user", user, "action", "iniciar copia", "error", err.Error())
			return map[string]any{"ok": false, "error": err.Error()}
		}
		s.audit(user, "iniciar copia")
		s.Publish()
		return map[string]any{"ok": true}
	default:
		return map[string]any{"ok": false, "error": "Petición no reconocida."}
	}
}

func (s *Server) pipeShow(user string, role Role, screen string) any {
	screen = strings.TrimSpace(screen)
	switch screen {
	case "", "historial", "restaurar", "ajustes", "copiar", "inicio":
	default:
		return map[string]any{"ok": false, "error": "Pantalla no reconocida."}
	}
	s.mu.Lock()
	liveSSE := len(s.subs) > 0
	s.mu.Unlock()
	if liveSSE && screen != "" && screen != "inicio" {
		raw, _ := json.Marshal(map[string]string{"navigate": screen})
		s.broadcast(raw)
	}
	// Solo reutilizar si hay un cliente SSE real (pestaña viva).
	// Una sesión huérfana sin navegador hacía que la bandeja no abriera nada.
	if liveSSE {
		return map[string]any{"ok": true, "reused": true, "user": user, "role": role}
	}
	token := s.IssueToken(user, role)
	s.log.Info("Token del panel emitido", "user", user, "role", role)
	url := s.LoginURL(token)
	if screen == "historial" || screen == "restaurar" || screen == "ajustes" || screen == "copiar" {
		url += "&next=" + screen
	}
	return map[string]any{"ok": true, "reused": false, "url": url, "user": user, "role": role}
}

func (s *Server) handleAdminProbe(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.sessionFrom(r)
	s.audit(sess.User, "comprobar acceso de administrador")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) mutating(next http.Handler) http.Handler {
	return s.withSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.originAllowed(r.Header.Get("Origin")) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "La petición no viene del panel."})
			return
		}
		sess, _ := s.sessionFrom(r)
		if subtleConstant(r.Header.Get("X-CSRF-Token"), sess.CSRF) == false {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "La petición no es válida. Vuelve a cargar el panel."})
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (s *Server) adminRead(next http.Handler) http.Handler {
	return s.withSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := s.sessionFrom(r)
		if sess.Role != RoleAdmin {
			s.log.Warn("Acción del panel rechazada", "user", sess.User, "action", r.URL.Path)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Hace falta un administrador."})
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (s *Server) admin(next http.Handler) http.Handler {
	return s.mutating(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := s.sessionFrom(r)
		if sess.Role != RoleAdmin {
			s.log.Warn("Acción del panel rechazada", "user", sess.User, "action", r.URL.Path)
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Hace falta un administrador."})
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.sessionFrom(r); !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Hace falta una sesión."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) redeem(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropExpiredLocked()
	ticket, ok := s.tokens[raw]
	if !ok || !s.now().Before(ticket.Expires) {
		delete(s.tokens, raw)
		return "", false
	}
	delete(s.tokens, raw)
	id := randomToken()
	s.sessions[id] = session{
		User:    ticket.User,
		Role:    ticket.Role,
		CSRF:    randomToken(),
		Expires: s.now().Add(sessionTTL),
	}
	return id, true
}

func (s *Server) sessionFrom(r *http.Request) (session, bool) {
	id := cookieValue(r)
	if id == "" {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropExpiredLocked()
	sess, ok := s.sessions[id]
	if !ok || !s.now().Before(sess.Expires) {
		delete(s.sessions, id)
		return session{}, false
	}
	return sess, true
}

func (s *Server) dropSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *Server) dropExpiredLocked() {
	now := s.now()
	for id, ticket := range s.tokens {
		if !now.Before(ticket.Expires) {
			delete(s.tokens, id)
		}
	}
	for id, sess := range s.sessions {
		if !now.Before(sess.Expires) {
			delete(s.sessions, id)
		}
	}
}

func (s *Server) audit(user, action string) {
	s.log.Info("Acción del panel", "user", user, "action", action)
}

func cookieValue(r *http.Request) string {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func randomToken() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

func subtleConstant(got, want string) bool {
	if len(got) != len(want) || want == "" {
		return false
	}
	var diff byte
	for i := range got {
		diff |= got[i] ^ want[i]
	}
	return diff == 0
}

func (s *Server) setNow(now func() time.Time) { s.now = now }

// ListenAddr es la única dirección en la que escucha el panel.
func ListenAddr(port int) string { return fmt.Sprintf("127.0.0.1:%d", port) }
