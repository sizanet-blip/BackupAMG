package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Códigos de negocio del contrato v1. Ninguno detiene las copias.
const (
	CodeInvalidLicense     = "invalid_license"
	CodeInvalidInstallCode = "invalid_install_code"
	CodeInstallRevoked     = "install_code_revoked"
	CodeInstallExpired     = "install_code_expired"
	CodeLicenseExpired     = "license_expired"
	CodeLicenseBlocked     = "license_blocked"
	CodeLicenseNotStarted  = "license_not_started"
	CodeMachineRevoked     = "machine_revoked"
	CodeSeatLimit          = "seat_limit_reached"
	CodeInvalidKey         = "invalid_key"
	CodeInvalidRequest     = "invalid_request"
	CodeRateLimited        = "rate_limited"
)

// maxResponse es lo máximo que se lee de una respuesta.
const maxResponse = 1 << 20

// APIError es una respuesta de error de la plataforma o un fallo al hablar con ella.
type APIError struct {
	// HTTPStatus es el código HTTP; 0 si no hubo respuesta (red caída, tiempo agotado).
	HTTPStatus int
	// Code es el código de máquina del contrato («seat_limit_reached», …). Vacío si no hubo respuesta.
	Code    string
	Message string
	Fields  []string
	// RetryAfter es lo que pide el servidor esperar (cabecera Retry-After o 429).
	RetryAfter time.Duration
	// Network es true si no se pudo hablar con el servidor.
	Network bool
	cause   error
}

func (e *APIError) Error() string {
	switch {
	case e.Network:
		return "no se pudo conectar con la plataforma"
	case e.Code != "":
		return "la plataforma respondió " + e.Code
	default:
		return "respuesta inesperada de la plataforma (" + strconv.Itoa(e.HTTPStatus) + ")"
	}
}

func (e *APIError) Unwrap() error { return e.cause }

// IsBusiness dice si el error es una decisión de la plataforma (licencia, plazas, revocado…): se
// muestra el estado y se vuelve a mirar dentro de unas horas, no con reintentos rápidos.
func (e *APIError) IsBusiness() bool {
	switch e.Code {
	case CodeInvalidLicense, CodeInvalidInstallCode, CodeInstallRevoked, CodeInstallExpired,
		CodeLicenseExpired, CodeLicenseBlocked, CodeLicenseNotStarted, CodeMachineRevoked,
		CodeSeatLimit, CodeInvalidKey:
		return true
	}
	return false
}

// IsRetryable dice si conviene reintentar con espera creciente: red, tiempo agotado, 429 y 5xx.
func (e *APIError) IsRetryable() bool {
	if e.IsBusiness() {
		return false
	}
	return e.Network || e.HTTPStatus == http.StatusTooManyRequests || e.HTTPStatus >= 500 || e.HTTPStatus == http.StatusRequestTimeout || e.HTTPStatus == 0
}

// asAPIError convierte cualquier error en *APIError.
func asAPIError(err error) *APIError {
	var api *APIError
	if errors.As(err, &api) {
		return api
	}
	return &APIError{Network: true, cause: err}
}

// Client habla con la API v1 de la plataforma. Solo HTTPS (http únicamente a localhost, para pruebas).
type Client struct {
	base string
	http *http.Client
}

// NewClient valida la dirección del servidor. server_url es la base, sin «/backup/v1».
func NewClient(serverURL string) (*Client, error) {
	base, err := normalizeBase(serverURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		base: base,
		http: &http.Client{
			Timeout: 30 * time.Second,
			// No se sigue ninguna redirección: la clave del equipo viaja en la cabecera Authorization.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// ErrBadServerURL es una dirección de servidor que no se puede usar.
var ErrBadServerURL = errors.New("la dirección del servidor no es válida: usa una dirección https, por ejemplo https://panel.ejemplo.com")

func normalizeBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", ErrBadServerURL
	}
	host := u.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	switch u.Scheme {
	case "https":
	case "http":
		if !local {
			return "", ErrBadServerURL
		}
	default:
		return "", ErrBadServerURL
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, "/backup/v1") // por si alguien pega la dirección completa
	return u.Scheme + "://" + u.Host + path, nil
}

// do envía un POST JSON. bearer vacío en el alta. out recibe la respuesta de éxito.
func (c *Client) do(ctx context.Context, route, bearer string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return &APIError{cause: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/backup/v1/"+route, bytes.NewReader(payload))
	if err != nil {
		return &APIError{Network: true, cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "BackupAMG-Agent")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return &APIError{Network: true, cause: scrubErr(err, bearer)}
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if readErr != nil {
		return &APIError{HTTPStatus: resp.StatusCode, Network: true, cause: readErr}
	}
	var envelope struct {
		OK      *bool    `json:"ok"`
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Fields  []string `json:"fields"`
	}
	parsed := json.Unmarshal(raw, &envelope) == nil && envelope.OK != nil
	if resp.StatusCode/100 == 2 && parsed && *envelope.OK {
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				return &APIError{HTTPStatus: resp.StatusCode, Code: "bad_response", cause: err}
			}
		}
		return nil
	}
	apiErr := &APIError{HTTPStatus: resp.StatusCode}
	if parsed {
		apiErr.Code, apiErr.Message, apiErr.Fields = envelope.Code, envelope.Message, envelope.Fields
	} else {
		apiErr.Code = "bad_response"
	}
	if secs, convErr := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); convErr == nil && secs > 0 {
		apiErr.RetryAfter = time.Duration(secs) * time.Second
	}
	return apiErr
}

// scrubErr evita que la clave del equipo aparezca en un mensaje de error de red.
func scrubErr(err error, secret string) error {
	if secret == "" {
		return err
	}
	var netErr net.Error
	if errors.As(err, &netErr) || strings.Contains(err.Error(), secret) {
		return errors.New(strings.ReplaceAll(err.Error(), secret, "***"))
	}
	return err
}

// ---- Tipos del contrato ---------------------------------------------------------------

// EnrollRequest es el alta. Va license_key O install_code, nunca los dos.
type EnrollRequest struct {
	LicenseKey    string `json:"license_key,omitempty"`
	InstallCode   string `json:"install_code,omitempty"`
	MachineUID    string `json:"machine_uid"`
	Hostname      string `json:"hostname"`
	OS            string `json:"os,omitempty"`
	AgentVersion  string `json:"agent_version"`
	ResticVersion string `json:"restic_version,omitempty"`
	ClientTime    string `json:"client_time,omitempty"`
	SchemaVersion int    `json:"schema_version"`
}

// LicenseInfo es lo que la plataforma cuenta de la licencia.
type LicenseInfo struct {
	Status           string `json:"status"`
	ValidUntil       string `json:"valid_until,omitempty"`
	MaxMachines      int    `json:"max_machines"`
	UsedMachines     int    `json:"used_machines"`
	OverLimit        bool   `json:"over_limit"`
	GraceDaysOffline int    `json:"grace_days_offline"`
	PlanName         string `json:"plan_name,omitempty"`
}

// EnrollResponse es la respuesta de éxito del alta.
type EnrollResponse struct {
	Code       string `json:"code"` // enrolled | re_enrolled
	MachineKey string `json:"machine_key"`
	Company    struct {
		Name string `json:"name"`
	} `json:"company"`
	License                 LicenseInfo `json:"license"`
	HeartbeatIntervalSecs   int         `json:"heartbeat_interval_seconds"`
	MinAgentVersion         string      `json:"min_agent_version"`
	RecommendedAgentVersion string      `json:"recommended_agent_version"`
	ServerTime              string      `json:"server_time"`
}

// Destination es el estado de la copia principal o de una copia adicional.
type Destination struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	State     string `json:"state"`
	LastOKAt  string `json:"last_ok_at,omitempty"`
	UsedBytes int64  `json:"used_bytes,omitempty"`
}

// Heartbeat es el latido. Todo es opcional salvo State.
type Heartbeat struct {
	State          string        `json:"state"`
	StateSince     string        `json:"state_since,omitempty"`
	ClientTime     string        `json:"client_time"`
	Hostname       string        `json:"hostname,omitempty"`
	OS             string        `json:"os,omitempty"`
	AgentVersion   string        `json:"agent_version,omitempty"`
	ResticVersion  string        `json:"restic_version,omitempty"`
	LastBackupAt   *string       `json:"last_backup_at"`
	LastSuccessAt  *string       `json:"last_success_at"`
	NextBackupAt   *string       `json:"next_backup_at"`
	LastVerifyAt   *string       `json:"last_verify_at"`
	LastVerifyOK   *bool         `json:"last_verify_ok"`
	VerifyDue      bool          `json:"verify_due"`
	DestFreeBytes  int64         `json:"dest_free_bytes"`
	DestTotalBytes int64         `json:"dest_total_bytes"`
	QueuePending   int           `json:"queue_pending"`
	Destinations   []Destination `json:"destinations,omitempty"`
}

// HeartbeatResponse es la respuesta del latido.
type HeartbeatResponse struct {
	Machine struct {
		Status string `json:"status"`
	} `json:"machine"`
	License                 LicenseInfo `json:"license"`
	HeartbeatIntervalSecs   int         `json:"heartbeat_interval_seconds"`
	MinAgentVersion         string      `json:"min_agent_version"`
	RecommendedAgentVersion string      `json:"recommended_agent_version"`
	ServerTime              string      `json:"server_time"`
}

// ReportResult es el resultado de un informe en el envío por lotes.
type ReportResult struct {
	ReportUID string `json:"report_uid"`
	Result    string `json:"result"` // stored | duplicate | rejected
	Code      string `json:"code,omitempty"`
}

// ReportsResponse es la respuesta del envío de informes.
type ReportsResponse struct {
	Results []ReportResult `json:"results"`
}

// Enroll da de alta el equipo.
func (c *Client) Enroll(ctx context.Context, req EnrollRequest) (EnrollResponse, error) {
	var out EnrollResponse
	if err := c.do(ctx, "enroll", "", req, &out); err != nil {
		return out, err
	}
	if !strings.HasPrefix(out.MachineKey, "bkm_") {
		return out, &APIError{HTTPStatus: 200, Code: "bad_response"}
	}
	return out, nil
}

// SendHeartbeat envía el latido con la clave del equipo.
func (c *Client) SendHeartbeat(ctx context.Context, key string, hb Heartbeat) (HeartbeatResponse, error) {
	var out HeartbeatResponse
	err := c.do(ctx, "heartbeat", key, hb, &out)
	return out, err
}

// SendReports envía informes ya saneados (cada uno es un JSON).
func (c *Client) SendReports(ctx context.Context, key string, reports []json.RawMessage) (ReportsResponse, error) {
	var out ReportsResponse
	err := c.do(ctx, "reports", key, map[string]any{"reports": reports}, &out)
	return out, err
}
