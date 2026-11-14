//go:build windows

package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"unsafe"

	"backup-agent/internal/status"
	"golang.org/x/sys/windows"
)

const (
	pipeAccessDuplex = 0x00000003
	pipeReadmodeByte = 0x00000000
	pipeTypeByte     = 0x00000000
	pipeWait         = 0x00000000
	pipeUnlimited    = 255
)

var (
	impersonateNamedPipeClient = windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateNamedPipeClient")
	waitNamedPipe              = windows.NewLazySystemDLL("kernel32.dll").NewProc("WaitNamedPipeW")
)

func (s *Server) servePipe(ctx context.Context) {
	for ctx.Err() == nil {
		handle, err := createPipe()
		if err != nil {
			s.log.Error("No se pudo abrir el canal de la bandeja", "error", err.Error())
			return
		}
		connected := make(chan error, 1)
		go func() {
			err := windows.ConnectNamedPipe(handle, nil)
			connected <- err
		}()
		select {
		case <-ctx.Done():
			_ = windows.CloseHandle(handle)
			<-connected
			return
		case err := <-connected:
			if err != nil && err != windows.ERROR_PIPE_CONNECTED {
				_ = windows.CloseHandle(handle)
				continue
			}
			go s.handlePipe(handle)
		}
	}
}

func createPipe() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(PipeName)
	if err != nil {
		return 0, err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)")
	if err != nil {
		return 0, err
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	return windows.CreateNamedPipe(name, pipeAccessDuplex, pipeTypeByte|pipeReadmodeByte|pipeWait, pipeUnlimited, 64*1024, 64*1024, 0, sa)
}

func (s *Server) handlePipe(handle windows.Handle) {
	defer windows.CloseHandle(handle)
	buf, err := readMessage(handle)
	if err != nil || len(buf) == 0 {
		return
	}
	user, role, err := clientIdentity(handle)
	if err != nil {
		s.log.Warn("No se pudo identificar quién abre el panel", "error", err.Error())
		writePipe(handle, map[string]any{"ok": false, "error": "No se pudo identificar al usuario de Windows."})
		return
	}
	var req struct {
		Op string `json:"op"`
	}
	if json.Unmarshal(buf, &req) != nil {
		writePipe(handle, map[string]any{"ok": false, "error": "Petición no reconocida."})
		return
	}
	writePipe(handle, s.pipeResult(user, role, req.Op))
}

func writePipe(handle windows.Handle, body any) {
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	raw = append(raw, '\n')
	_ = writeAll(handle, raw)
}

func writeAll(handle windows.Handle, raw []byte) error {
	for len(raw) > 0 {
		var written uint32
		if err := windows.WriteFile(handle, raw, &written, nil); err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		raw = raw[written:]
	}
	return nil
}

func readMessage(handle windows.Handle) ([]byte, error) {
	var all []byte
	buf := make([]byte, 4096)
	for len(all) < 64*1024 {
		var read uint32
		err := windows.ReadFile(handle, buf, &read, nil)
		if read > 0 {
			all = append(all, buf[:read]...)
			if bytes.Contains(all, []byte("\n")) {
				return all, nil
			}
		}
		if err != nil {
			if len(all) > 0 {
				return all, nil
			}
			return nil, err
		}
		if read == 0 {
			break
		}
	}
	return all, nil
}

func clientIdentity(handle windows.Handle) (string, Role, error) {
	if err := impersonate(handle); err != nil {
		return "", "", err
	}
	defer windows.RevertToSelf()

	var token windows.Token
	err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token)
	if err != nil {
		return "", "", err
	}
	defer token.Close()

	info, err := token.GetTokenUser()
	if err != nil {
		return "", "", err
	}
	account, domain, _, err := info.User.Sid.LookupAccount("")
	if err != nil {
		return "", "", err
	}
	user := account
	if domain != "" {
		user = domain + `\` + account
	}
	role := RoleUser
	if memberOfAdministrators(token) {
		role = RoleAdmin
	}
	return user, role, nil
}

func memberOfAdministrators(token windows.Token) bool {
	sid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	if ok, err := token.IsMember(sid); err == nil && ok {
		return true
	}
	linked, err := token.GetLinkedToken()
	if err != nil {
		return false
	}
	defer linked.Close()
	ok, err := linked.IsMember(sid)
	return err == nil && ok
}

func impersonate(handle windows.Handle) error {
	r, _, err := impersonateNamedPipeClient.Call(uintptr(handle))
	if r == 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return err
		}
		return fmt.Errorf("no se pudo identificar al cliente del canal")
	}
	return nil
}

// RequestToken pide al agente en marcha un enlace de un solo uso.
func RequestToken() (url, user string, role Role, err error) {
	resp, err := callPipe("token")
	if err != nil {
		return "", "", "", err
	}
	return resp.URL, resp.User, resp.Role, nil
}

// RequestStatus lee el semáforo por el canal de la bandeja.
func RequestStatus() (status.View, error) {
	resp, err := callPipe("status")
	if err != nil {
		return status.View{}, err
	}
	var view status.View
	if len(resp.Status) > 0 {
		if err := json.Unmarshal(resp.Status, &view); err != nil {
			return status.View{}, err
		}
	}
	return view, nil
}

// RequestBackup pide una copia inmediata. No abre el panel.
func RequestBackup() error {
	_, err := callPipe("backup")
	return err
}

func callPipe(op string) (pipeResponse, error) {
	handle, err := dialPipe()
	if err != nil {
		return pipeResponse{}, fmt.Errorf("el agente no está en marcha. Ejecuta agent run o inicia el servicio: %w", err)
	}
	defer windows.CloseHandle(handle)
	payload, err := json.Marshal(map[string]string{"op": op})
	if err != nil {
		return pipeResponse{}, err
	}
	payload = append(payload, '\n')
	if err := writeAll(handle, payload); err != nil {
		return pipeResponse{}, err
	}
	raw, err := readMessage(handle)
	if err != nil {
		return pipeResponse{}, err
	}
	var resp pipeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return pipeResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "El agente no ha aceptado la petición."
		}
		return pipeResponse{}, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

type pipeResponse struct {
	OK     bool            `json:"ok"`
	URL    string          `json:"url"`
	User   string          `json:"user"`
	Role   Role            `json:"role"`
	Error  string          `json:"error"`
	Status json.RawMessage `json:"status"`
}

func dialPipe() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(PipeName)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err == nil {
		return handle, nil
	}
	if err == windows.ERROR_PIPE_BUSY {
		waitNamedPipe.Call(uintptr(unsafe.Pointer(name)), 2000)
		return windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	}
	return 0, err
}
