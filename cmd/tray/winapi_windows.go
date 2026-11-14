//go:build windows

package main

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	nifMessage    = 0x00000001
	nifIcon       = 0x00000002
	nifTip        = 0x00000004
	nifInfo       = 0x00000010
	nifGuid       = 0x00000020
	nifShowTip    = 0x00000080
	nimAdd        = 0x00000000
	nimModify     = 0x00000001
	nimDelete     = 0x00000002
	nimSetVersion = 0x00000004
	niifInfo      = 0x00000001
	notifyVersion4 = 4
)

// Layout alineado con NOTIFYICONDATAW de Windows (x64).
// uTimeout y uVersion comparten el mismo campo.
type notifyIconData struct {
	Size            uint32
	Wnd             windows.Handle
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            windows.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Timeout         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        windows.GUID
	BalloonIcon     windows.Handle
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type point struct {
	X, Y int32
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type paintStruct struct {
	HDC         windows.Handle
	Erase       int32
	RcPaint     rect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type msg struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

type iconInfo struct {
	Icon     int32
	XHotspot uint32
	YHotspot uint32
	Mask     windows.Handle
	Color    windows.Handle
}

type notifyIconIdentifier struct {
	Size     uint32
	Wnd      windows.Handle
	ID       uint32
	GuidItem windows.GUID
}

type appBarData struct {
	Size            uint32
	Wnd             windows.Handle
	CallbackMessage uint32
	Edge            uint32
	Rc              rect
	LParam          int32
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pGetModuleHandle               = kernel32.NewProc("GetModuleHandleW")
	pRegisterClass                 = user32.NewProc("RegisterClassExW")
	pCreateWindowEx                = user32.NewProc("CreateWindowExW")
	pDefWindowProc                 = user32.NewProc("DefWindowProcW")
	pDestroyWindow                 = user32.NewProc("DestroyWindow")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pGetMessage                    = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessage               = user32.NewProc("DispatchMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pPostMessage                   = user32.NewProc("PostMessageW")
	pLoadCursor                    = user32.NewProc("LoadCursorW")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pGetCursorPos                  = user32.NewProc("GetCursorPos")
	pGetWindowRect                 = user32.NewProc("GetWindowRect")
	pGetDC                         = user32.NewProc("GetDC")
	pReleaseDC                     = user32.NewProc("ReleaseDC")
	pBeginPaint                    = user32.NewProc("BeginPaint")
	pEndPaint                      = user32.NewProc("EndPaint")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pGetClientRect                 = user32.NewProc("GetClientRect")
	pFillRect                      = user32.NewProc("FillRect")
	pDrawText                      = user32.NewProc("DrawTextW")
	pMonitorFromPoint              = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfo                = user32.NewProc("GetMonitorInfoW")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	pIsWindowVisible               = user32.NewProc("IsWindowVisible")
	pRegisterWindowMessage         = user32.NewProc("RegisterWindowMessageW")
	pSystemParametersInfo          = user32.NewProc("SystemParametersInfoW")
	pSetLayeredWindowAttributes    = user32.NewProc("SetLayeredWindowAttributes")
	pGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	pRegisterHotKey                = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey              = user32.NewProc("UnregisterHotKey")
	pSetTimer                      = user32.NewProc("SetTimer")
	pKillTimer                     = user32.NewProc("KillTimer")
	pGetAsyncKeyState              = user32.NewProc("GetAsyncKeyState")
	pCreateSolidBrush              = gdi32.NewProc("CreateSolidBrush")
	pCreatePen                     = gdi32.NewProc("CreatePen")
	pSelectObject                  = gdi32.NewProc("SelectObject")
	pDeleteObject                  = gdi32.NewProc("DeleteObject")
	pSetBkMode                     = gdi32.NewProc("SetBkMode")
	pSetTextColor                  = gdi32.NewProc("SetTextColor")
	pCreateFont                    = gdi32.NewProc("CreateFontW")
	pRoundRect                     = gdi32.NewProc("RoundRect")
	pEllipse                       = gdi32.NewProc("Ellipse")
	pMoveToEx                      = gdi32.NewProc("MoveToEx")
	pLineTo                        = gdi32.NewProc("LineTo")
	pCreateCompatibleDC            = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection              = gdi32.NewProc("CreateDIBSection")
	pCreateCompatibleBitmap        = gdi32.NewProc("CreateCompatibleBitmap")
	pDeleteDC                      = gdi32.NewProc("DeleteDC")
	pCreateIconIndirect            = user32.NewProc("CreateIconIndirect")
	pGetStockObject                = gdi32.NewProc("GetStockObject")
	pShellNotifyIcon               = shell32.NewProc("Shell_NotifyIconW")
	pShellNotifyIconGetRect        = shell32.NewProc("Shell_NotifyIconGetRect")
	pSHAppBarMessage               = shell32.NewProc("SHAppBarMessage")
)

func setDPIAware() {
	const perMonitorV2 = ^uintptr(3) // -4
	_, _, _ = pSetProcessDpiAwarenessContext.Call(perMonitorV2)
}

func loadCursor() windows.Handle {
	h, _, _ := pLoadCursor.Call(0, 32512)
	return windows.Handle(h)
}

func shellNotify(action uint32, nid *notifyIconData) error {
	nid.Size = uint32(unsafe.Sizeof(*nid))
	res, _, err := pShellNotifyIcon.Call(uintptr(action), uintptr(unsafe.Pointer(nid)))
	if res == 0 {
		return err
	}
	return nil
}

func shellNotifyGetRect(id *notifyIconIdentifier, rc *rect) error {
	id.Size = uint32(unsafe.Sizeof(*id))
	res, _, err := pShellNotifyIconGetRect.Call(uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(rc)))
	if res != 0 {
		if err != nil && err != windows.ERROR_SUCCESS {
			return err
		}
		return windows.Errno(res)
	}
	return nil
}

func taskbarRect() (rect, bool) {
	var data appBarData
	data.Size = uint32(unsafe.Sizeof(data))
	const abmGetTaskbarPos = 5
	res, _, _ := pSHAppBarMessage.Call(abmGetTaskbarPos, uintptr(unsafe.Pointer(&data)))
	if res == 0 || data.Rc.Right <= data.Rc.Left || data.Rc.Bottom <= data.Rc.Top {
		return rect{}, false
	}
	return data.Rc, true
}

func nativeLoop(windows.Handle) {
	var m msg
	for {
		ret, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(ret) {
		case -1, 0:
			return
		default:
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}

func copyUTF16(dest []uint16, value string) {
	encoded, err := windows.UTF16FromString(value)
	if err != nil || len(dest) == 0 {
		return
	}
	if len(encoded) > len(dest) {
		encoded = encoded[:len(dest)]
		encoded[len(encoded)-1] = 0
	}
	copy(dest, encoded)
}

func nowLocal() time.Time { return time.Now() }

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func iconHandle(ico []byte) windows.Handle {
	if len(ico) < 22 {
		return 0
	}
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		return 0
	}
	defer pReleaseDC.Call(0, hdc)

	const size = 32
	bmi := bitmapInfo{
		Header: bitmapInfoHeader{
			Size:     40,
			Width:    size,
			Height:   -size,
			Planes:   1,
			BitCount: 32,
		},
	}
	var bits unsafe.Pointer
	hbmColor, _, _ := pCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbmColor == 0 || bits == nil {
		return 0
	}
	offset := 22 + 40
	if len(ico) < offset+size*size*4 {
		pDeleteObject.Call(hbmColor)
		return 0
	}
	dst := unsafe.Slice((*byte)(bits), size*size*4)
	src := ico[offset : offset+size*size*4]
	for y := 0; y < size; y++ {
		copy(dst[y*size*4:(y+1)*size*4], src[(size-1-y)*size*4:(size-y)*size*4])
	}
	hbmMask, _, _ := pCreateCompatibleBitmap.Call(hdc, size, size)
	info := iconInfo{Icon: 1, Mask: windows.Handle(hbmMask), Color: windows.Handle(hbmColor)}
	hicon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	pDeleteObject.Call(hbmColor)
	if hbmMask != 0 {
		pDeleteObject.Call(hbmMask)
	}
	return windows.Handle(hicon)
}

func scale(value, dpi int) int {
	return value * dpi / 96
}
