//go:build windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	enumWindows          = user32.NewProc("EnumWindows")
	getWindowTextW       = user32.NewProc("GetWindowTextW")
	getWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	isWindowVisible      = user32.NewProc("IsWindowVisible")
	isWindow             = user32.NewProc("IsWindow")
	getWindow            = user32.NewProc("GetWindow")
	showWindow           = user32.NewProc("ShowWindow")
	setForegroundWindow  = user32.NewProc("SetForegroundWindow")
	bringWindowToTop     = user32.NewProc("BringWindowToTop")
	getWindowThreadProc  = user32.NewProc("GetWindowThreadProcessId")
	allowSetForeground   = user32.NewProc("AllowSetForegroundWindow")
)

const (
	gwOwner    = 4
	swRestore  = 9
	asfwAny    = 0xffffffff
	panelTitle = "BackupAMG"
)

var browserExes = map[string]struct{}{
	"chrome.exe":         {},
	"msedge.exe":         {},
	"firefox.exe":        {},
	"brave.exe":          {},
	"opera.exe":          {},
	"opera_gx.exe":       {},
	"vivaldi.exe":        {},
	"iexplore.exe":       {},
	"chromium.exe":       {},
	"waterfox.exe":       {},
	"librewolf.exe":      {},
	"browser.exe":        {},
	"msedgewebview2.exe": {},
}

var (
	rememberedPanelHWND atomic.Uintptr
	enumCbOnce          sync.Once
	enumPanelCb         uintptr
	enumMu              sync.Mutex
	enumFindState       struct {
		found uintptr
	}
)

// OpenURL abre el panel en una ventana de aplicación (--app) del navegador
// predeterminado de Windows. Así no se mezcla con otras pestañas.
func OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("la dirección del panel está vacía")
	}
	if FocusPanelWindow() {
		return nil
	}
	browser, err := findDefaultBrowserExe()
	if err != nil {
		return err
	}
	profile, err := panelProfileDir()
	if err != nil {
		return err
	}
	cmd := exec.Command(browser,
		"--user-data-dir="+profile,
		"--app="+url,
		"--disable-extensions",
		"--no-first-run",
		"--no-default-browser-check",
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo abrir el panel: %w", err)
	}
	_ = cmd.Process.Release()
	go RememberPanelWindowSoon()
	return nil
}

// FocusPanelWindow busca la ventana de aplicación del panel y la trae al frente.
func FocusPanelWindow() bool {
	hwnd := findPanelBrowserWindow()
	if hwnd == 0 {
		hwnd = rememberedPanelHWND.Load()
		if hwnd == 0 || !windowAlive(hwnd) {
			return false
		}
	}
	rememberedPanelHWND.Store(hwnd)
	focusWindow(hwnd)
	return true
}

// FocusBrowserForPanel intenta enfocar la ventana del panel.
func FocusBrowserForPanel() bool {
	return FocusPanelWindow()
}

// RememberPanelWindowSoon espera a que aparezca la ventana BackupAMG y guarda su HWND.
func RememberPanelWindowSoon() {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if hwnd := findPanelBrowserWindow(); hwnd != 0 {
			rememberedPanelHWND.Store(hwnd)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func panelProfileDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "BackupAMG", "panel-app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func findDefaultBrowserExe() (string, error) {
	if exe := browserExeFromUserChoice(); exe != "" {
		return exe, nil
	}
	// Respaldo si no hay asociación legible.
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		if root == "" {
			continue
		}
		for _, rel := range []string{
			filepath.Join("Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
		} {
			path := filepath.Join(root, rel)
			if st, err := os.Stat(path); err == nil && !st.IsDir() {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("no se encontró un navegador")
}

func browserExeFromUserChoice() string {
	for _, scheme := range []string{"https", "http"} {
		keyPath := `Software\Microsoft\Windows\Shell\Associations\UrlAssociations\` + scheme + `\UserChoice`
		k, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		progID, _, err := k.GetStringValue("ProgId")
		k.Close()
		if err != nil || progID == "" {
			continue
		}
		if exe := exeFromProgID(progID); exe != "" {
			return exe
		}
	}
	return ""
}

func exeFromProgID(progID string) string {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `Software\Classes\`+progID+`\shell\open\command`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		cmd, _, err := k.GetStringValue("")
		k.Close()
		if err != nil || cmd == "" {
			continue
		}
		if exe := firstQuotedPath(cmd); exe != "" {
			if st, err := os.Stat(exe); err == nil && !st.IsDir() {
				return exe
			}
		}
	}
	return ""
}

func firstQuotedPath(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.HasPrefix(cmd, `"`) {
		end := strings.Index(cmd[1:], `"`)
		if end >= 0 {
			return cmd[1 : 1+end]
		}
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func ensureEnumCallbacks() {
	enumCbOnce.Do(func() {
		enumPanelCb = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
			if !visibleTopLevel(hwnd) {
				return 1
			}
			title := windowTitle(hwnd)
			if !strings.HasPrefix(title, panelTitle) {
				return 1
			}
			var pid uint32
			getWindowThreadProc.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if !isBrowserProcess(pid) {
				return 1
			}
			enumFindState.found = hwnd
			return 0
		})
	})
}

func findPanelBrowserWindow() uintptr {
	ensureEnumCallbacks()
	enumMu.Lock()
	defer enumMu.Unlock()
	enumFindState.found = 0
	enumWindows.Call(enumPanelCb, 0)
	return enumFindState.found
}

func visibleTopLevel(hwnd uintptr) bool {
	vis, _, _ := isWindowVisible.Call(hwnd)
	if vis == 0 {
		return false
	}
	owner, _, _ := getWindow.Call(hwnd, gwOwner)
	return owner == 0
}

func windowAlive(hwnd uintptr) bool {
	r, _, _ := isWindow.Call(hwnd)
	return r != 0
}

func isBrowserProcess(pid uint32) bool {
	if pid == 0 {
		return false
	}
	const processQueryLimitedInformation = 0x1000
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var buf [windows.MAX_PATH]uint16
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return false
	}
	name := strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:n])))
	_, ok := browserExes[name]
	return ok
}

func windowTitle(hwnd uintptr) string {
	n, _, _ := getWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	getWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(n+1))
	return windows.UTF16ToString(buf)
}

func focusWindow(hwnd uintptr) {
	allowSetForeground.Call(asfwAny)
	showWindow.Call(hwnd, swRestore)
	bringWindowToTop.Call(hwnd)
	setForegroundWindow.Call(hwnd)
}
