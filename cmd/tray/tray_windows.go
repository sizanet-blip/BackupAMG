//go:build windows

package main

import (
	"image/color"
	"sync"
	"time"
	"unsafe"

	"backup-agent/internal/panel"
	"backup-agent/internal/status"
	"golang.org/x/sys/windows"
)

const (
	trayHostClass     = "BackupAMGTrayHost"
	trayIconID        = 1
	wmTray            = 0x0400 + 32
	wmAppRefresh      = 0x8000 + 1
	wmAppQuit         = 0x8000 + 2
	wmAppShow         = 0x8000 + 3
	wmAppPopupRepaint = 0x8000 + 4
	wmAppApplyStatus  = 0x8000 + 5
	wmAppToast        = 0x8000 + 6
)

// Sin GUID fijo: con GUID Windows deja iconos «zombies» de ejecuciones anteriores
// que se ven en la bandeja pero no responden al clic.
var oldTrayGUIDs = []windows.GUID{
	{Data1: 0x9F1072EA, Data2: 0x11FF, Data3: 0x416A, Data4: [8]byte{0xB5, 0x38, 0x48, 0xC0, 0xA9, 0x5D, 0xB6, 0x90}},
	{Data1: 0x9F1072EA, Data2: 0x11FF, Data3: 0x416A, Data4: [8]byte{0xB5, 0x38, 0x48, 0xC0, 0xA9, 0x5D, 0xB6, 0x91}},
}

type trayApp struct {
	mu           sync.Mutex
	hwnd         windows.Handle
	icons        map[string]windows.Handle
	iconKey      string
	online       bool
	busy         bool
	title        string
	detail       string
	view         status.View
	prev         *status.View
	nid          notifyIconData
	taskbar      uint32
	popup        *popupWindow
	running      bool
	pendingView  status.View
	pendingErr   error
	pendingReady bool
	toastTitle   string
	toastBody    string
	toastReady   bool
	lastToggle   time.Time
	iconRC       rect
	iconRCOK     bool
}

var activeTray *trayApp

func (t *trayApp) run() {
	setDPIAware()
	activeTray = t
	defer func() { activeTray = nil }()
	if err := t.register(); err != nil {
		return
	}
	t.icons = map[string]windows.Handle{
		status.Green:  iconHandle(circleICO(color.NRGBA{R: 23, G: 121, B: 74, A: 255})),
		status.Yellow: iconHandle(circleICO(color.NRGBA{R: 224, G: 161, B: 0, A: 255})),
		status.Red:    iconHandle(circleICO(color.NRGBA{R: 168, G: 36, B: 26, A: 255})),
		status.Blue:   iconHandle(circleICO(color.NRGBA{R: 35, G: 80, B: 216, A: 255})),
		"gray":        iconHandle(circleICO(color.NRGBA{R: 138, G: 144, B: 155, A: 255})),
	}
	t.title = "Servicio parado"
	t.detail = "Las copias no se están haciendo"
	t.iconKey = "gray"
	t.popup = newPopup(t)
	clearStaleTrayIcons()
	if err := t.addIcon(); err != nil {
		return
	}
	promoteTrayIcon()
	t.running = true
	go t.refreshBackground()
	nativeLoop(t.hwnd)
	t.removeIcon()
	t.running = false
}

func (t *trayApp) register() error {
	instance, _, err := pGetModuleHandle.Call(0)
	if instance == 0 {
		return err
	}
	classPtr, err := windows.UTF16PtrFromString(trayHostClass)
	if err != nil {
		return err
	}
	namePtr, err := windows.UTF16PtrFromString("BackupAMG")
	if err != nil {
		return err
	}
	wcex := wndClassEx{
		Style:     0x0002 | 0x0001,
		WndProc:   windows.NewCallback(t.wndProc),
		Instance:  windows.Handle(instance),
		Cursor:    loadCursor(),
		ClassName: classPtr,
	}
	wcex.Size = uint32(unsafe.Sizeof(wcex))
	if res, _, err := pRegisterClass.Call(uintptr(unsafe.Pointer(&wcex))); res == 0 {
		return err
	}
	taskbarName, _ := windows.UTF16PtrFromString("TaskbarCreated")
	msg, _, _ := pRegisterWindowMessage.Call(uintptr(unsafe.Pointer(taskbarName)))
	t.taskbar = uint32(msg)

	hwnd, _, err := pCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(namePtr)),
		0x00CF0000,
		0x80000000, 0x80000000, 0x80000000, 0x80000000,
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return err
	}
	t.hwnd = windows.Handle(hwnd)
	pShowWindow.Call(hwnd, 0)
	return nil
}

func clearStaleTrayIcons() {
	for _, g := range oldTrayGUIDs {
		nid := notifyIconData{Flags: nifGuid, GuidItem: g}
		_ = shellNotify(nimDelete, &nid)
	}
}

func (t *trayApp) addIcon() error {
	t.mu.Lock()
	icon := t.icons[t.iconKey]
	if icon == 0 {
		icon = t.icons["gray"]
	}
	t.nid = notifyIconData{
		Wnd:             t.hwnd,
		ID:              trayIconID,
		Flags:           nifMessage | nifIcon | nifTip | nifShowTip,
		CallbackMessage: wmTray,
		Icon:            icon,
	}
	copyUTF16(t.nid.Tip[:], tooltipText(t.title, t.detail))
	nid := t.nid
	t.mu.Unlock()
	// Nunca Shell_NotifyIcon con el mutex cogido: Explorer puede reentrar al wndProc.
	if err := shellNotify(nimAdd, &nid); err != nil {
		return err
	}
	ver := nid
	ver.Timeout = notifyVersion4
	ver.Flags = 0
	_ = shellNotify(nimSetVersion, &ver)
	nid.Flags = nifMessage | nifIcon | nifTip | nifShowTip
	t.mu.Lock()
	t.nid = nid
	t.mu.Unlock()
	return nil
}

func (t *trayApp) removeIcon() {
	t.mu.Lock()
	nid := t.nid
	hwnd := t.hwnd
	t.mu.Unlock()
	if hwnd == 0 {
		return
	}
	_ = shellNotify(nimDelete, &nid)
}

// applyIconLocked actualiza t.nid; el caller debe soltar mu antes de notifyIcon.
func (t *trayApp) applyIconLocked() {
	icon := t.icons[t.iconKey]
	if icon == 0 {
		icon = t.icons["gray"]
	}
	t.nid.Icon = icon
	t.nid.Flags = nifMessage | nifIcon | nifTip | nifShowTip
	copyUTF16(t.nid.Tip[:], tooltipText(t.title, t.detail))
}

func (t *trayApp) notifyIcon() {
	t.mu.Lock()
	nid := t.nid
	t.mu.Unlock()
	if nid.Wnd == 0 {
		return
	}
	_ = shellNotify(nimModify, &nid)
}

// refreshBackground pide estado fuera del hilo de UI.
// Shell_NotifyIcon solo se toca luego en applyPendingStatus (hilo de UI):
// mezclarlo con el timer del popup colgaba el icono.
func (t *trayApp) refreshBackground() {
	view, err := requestStatus()
	t.cacheIconRect()
	t.mu.Lock()
	t.pendingView = view
	t.pendingErr = err
	t.pendingReady = true
	hwnd := t.hwnd
	t.mu.Unlock()
	if hwnd != 0 {
		pPostMessage.Call(uintptr(hwnd), wmAppApplyStatus, 0, 0)
	}
}

func (t *trayApp) applyPendingStatus() {
	t.mu.Lock()
	if !t.pendingReady {
		t.mu.Unlock()
		return
	}
	view := t.pendingView
	err := t.pendingErr
	t.pendingReady = false

	if err != nil {
		t.online = false
		t.busy = false
		t.view = status.View{}
		t.title, t.detail = menuLines(status.View{}, false, nowLocal())
		t.iconKey = "gray"
		t.applyIconLocked()
		t.prev = nil
		t.mu.Unlock()
		t.notifyIcon()
		if t.popup != nil && t.popup.visible() {
			t.popup.repaint()
		}
		return
	}
	level := view.Level
	busy := view.Activity == status.ActivityBackup || view.Activity == status.ActivityVerify || view.Activity == status.ActivityRestore
	if busy {
		level = status.Blue
	}
	title, detail := menuLines(view, true, nowLocal())
	t.online = true
	t.busy = busy
	t.view = view
	t.title = title
	t.detail = detail
	t.iconKey = level
	if t.icons[t.iconKey] == 0 {
		t.iconKey = "gray"
	}
	t.applyIconLocked()
	noteTitle, body, noteOK := backupNotice(t.prev, view, noticeOptions{warnings: true})
	prev := view
	t.prev = &prev
	t.mu.Unlock()
	t.notifyIcon()
	if noteOK {
		pushToast(noteTitle, body)
	}
	if t.popup != nil && t.popup.visible() {
		t.popup.repaint()
	}
}

func (t *trayApp) applyPendingToast() {
	t.mu.Lock()
	if !t.toastReady {
		t.mu.Unlock()
		return
	}
	title := t.toastTitle
	body := t.toastBody
	t.toastReady = false
	nid := t.nid
	t.mu.Unlock()
	if nid.Wnd == 0 {
		return
	}
	nid.Flags = nifInfo
	nid.Timeout = 10000
	nid.InfoFlags = niifInfo
	copyUTF16(nid.InfoTitle[:], title)
	copyUTF16(nid.Info[:], body)
	_ = shellNotify(nimModify, &nid)
}

func (t *trayApp) snapshot() (title, detail string, online, busy bool, level string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title, t.detail, t.online, t.busy, t.iconKey
}

func (t *trayApp) wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTray:
		mouse := uint32(lParam & 0xFFFF)
		switch mouse {
		case 0x0202, 0x0205, 0x0203, 0x0206, // button up / dbl
			0x007B,         // WM_CONTEXTMENU
			0x0400, 0x0401: // NIN_SELECT / NIN_KEYSELECT
			t.togglePopup()
		}
		return 0
	case wmAppRefresh:
		go t.refreshBackground()
		return 0
	case wmAppApplyStatus:
		t.applyPendingStatus()
		return 0
	case wmAppToast:
		t.applyPendingToast()
		return 0
	case wmAppPopupRepaint:
		if t.popup != nil && t.popup.visible() {
			t.popup.repaint()
		}
		return 0
	case wmAppShow:
		t.showPopup()
		return 0
	case wmAppQuit:
		if t.popup != nil {
			t.popup.hide()
		}
		pDestroyWindow.Call(uintptr(hwnd))
		return 0
	case t.taskbar:
		_ = t.addIcon()
		go t.refreshBackground()
		return 0
	case 0x0010: // WM_CLOSE
		pDestroyWindow.Call(uintptr(hwnd))
		return 0
	case 0x0002: // WM_DESTROY
		pPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := pDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func (t *trayApp) togglePopup() {
	// Un clic físico a veces manda WM_LBUTTONUP y NIN_SELECT: sin filtro el popup
	// se cierra y se vuelve a abrir (parece que el icono «no hace nada»).
	if time.Since(t.lastToggle) < 350*time.Millisecond {
		return
	}
	t.lastToggle = time.Now()
	if t.popup != nil && t.popup.visible() {
		t.popup.hide()
		return
	}
	t.showPopup()
}

func (t *trayApp) showPopup() {
	// Solo datos en caché: pedir estado al agente aquí dejaría el icono pillado.
	if t.popup != nil {
		t.popup.show()
	}
}

// iconRect lee la posición guardada. No llama a la bandeja: hacerlo en el hilo
// de UI dejaba el icono pillado al abrir el panel.
func (t *trayApp) iconRect() (rect, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.iconRCOK {
		return rect{}, false
	}
	return t.iconRC, true
}

func (t *trayApp) cacheIconRect() {
	rc, ok := t.queryIconRect()
	t.mu.Lock()
	t.iconRC = rc
	t.iconRCOK = ok
	t.mu.Unlock()
}

func (t *trayApp) queryIconRect() (rect, bool) {
	t.mu.Lock()
	nid := t.nid
	t.mu.Unlock()
	id := notifyIconIdentifier{Wnd: nid.Wnd, ID: nid.ID}
	var rc rect
	if err := shellNotifyGetRect(&id, &rc); err != nil {
		return rect{}, false
	}
	if rc.Right <= rc.Left || rc.Bottom <= rc.Top {
		return rect{}, false
	}
	return rc, true
}

func tooltipText(title, detail string) string {
	if detail == "" {
		return title
	}
	text := title + " · " + detail
	if len([]rune(text)) > 120 {
		return title
	}
	return text
}

func requestStatus() (status.View, error) {
	return panel.RequestStatus()
}
