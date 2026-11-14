//go:build windows

package main

import (
	"time"
	"unsafe"

	"backup-agent/internal/panel"
	"backup-agent/internal/status"
	"golang.org/x/sys/windows"
)

const (
	popupClass   = "BackupAMGTrayPopup"
	popupTimerID = 1
	popupTimerMs = 50
	wmTimer      = 0x0113
	vkLeftButton = 0x01
)

type popupWindow struct {
	app        *trayApp
	hwnd       windows.Handle
	dpi        int
	hover      int
	shown      bool
	rows       []popupRow
	actions    []popupAction
	closeBtn   rect
	openedAt   time.Time
	prevLeft   bool
	iconCache  rect
	iconCacheOK bool
}

type popupRow struct {
	kind   int // 0 header, 1 action, 2 hide, 3 legend, 4 gap
	bounds rect
	action int
}

type popupAction struct {
	label    string
	enabled  bool
	handler  func()
}

func newPopup(app *trayApp) *popupWindow {
	p := &popupWindow{app: app, dpi: 96, hover: -1}
	_ = p.create()
	return p
}

func (p *popupWindow) create() error {
	instance, _, err := pGetModuleHandle.Call(0)
	if instance == 0 {
		return err
	}
	classPtr, err := windows.UTF16PtrFromString(popupClass)
	if err != nil {
		return err
	}
	wcex := wndClassEx{
		Style:      0x0002 | 0x0001,
		WndProc:    windows.NewCallback(p.wndProc),
		Instance:   windows.Handle(instance),
		Cursor:     loadCursor(),
		Background: brush(rgb(245, 246, 248)),
		ClassName:  classPtr,
	}
	wcex.Size = uint32(unsafe.Sizeof(wcex))
	_, _, _ = pRegisterClass.Call(uintptr(unsafe.Pointer(&wcex)))

	const wsPopup = 0x80000000
	const wsExTool = 0x00000080
	const wsExNoAct = 0x08000000
	// Sin TOPMOST ni activación: no tapa Impr Pant / recorte y el icono sigue clicable.
	hwnd, _, err := pCreateWindowEx.Call(
		wsExTool|wsExNoAct,
		uintptr(unsafe.Pointer(classPtr)),
		0,
		wsPopup,
		0, 0, 10, 10,
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return err
	}
	p.hwnd = windows.Handle(hwnd)
	return nil
}

func (p *popupWindow) visible() bool {
	if p == nil || p.hwnd == 0 {
		return false
	}
	v, _, _ := pIsWindowVisible.Call(uintptr(p.hwnd))
	return v != 0
}

func (p *popupWindow) show() {
	if p.hwnd == 0 {
		return
	}
	p.rebuild()
	w, h := p.layout()
	x, y := p.anchorNearTray(w, h)
	const swpShow = 0x0040
	const swpNoActivate = 0x0010
	p.hover = -1
	p.openedAt = time.Now()
	p.prevLeft = true // el clic que abre no debe cerrar al soltar
	p.refreshIconCache()
	// TOPMOST solo mientras está abierta.
	const hwndTopmost = ^uintptr(0)
	pSetWindowPos.Call(uintptr(p.hwnd), hwndTopmost, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpShow|swpNoActivate)
	pShowWindow.Call(uintptr(p.hwnd), 4) // SW_SHOWNOACTIVATE
	p.shown = true
	pRegisterHotKey.Call(uintptr(p.hwnd), 1, 0, 0x1B) // Escape
	// Solo botón izquierdo del ratón: la barra de tareas no entrega clics a ventanas encima.
	pSetTimer.Call(uintptr(p.hwnd), popupTimerID, popupTimerMs, 0)
	pInvalidateRect.Call(uintptr(p.hwnd), 0, 1)
}

func (p *popupWindow) refreshIconCache() {
	if p == nil || p.app == nil {
		return
	}
	p.iconCache, p.iconCacheOK = p.app.iconRect()
}

func (p *popupWindow) hide() {
	if p.hwnd == 0 || !p.shown {
		return
	}
	pKillTimer.Call(uintptr(p.hwnd), popupTimerID)
	pUnregisterHotKey.Call(uintptr(p.hwnd), 1)
	const hwndNoTopmost = ^uintptr(1)
	const swpHide = 0x0080
	const swpNoActivate = 0x0010
	const swpNoMove = 0x0002
	const swpNoSize = 0x0001
	pSetWindowPos.Call(uintptr(p.hwnd), hwndNoTopmost, 0, 0, 0, 0, swpHide|swpNoActivate|swpNoMove|swpNoSize)
	p.shown = false
	p.hover = -1
}

func (p *popupWindow) stillWarm() bool {
	return time.Since(p.openedAt) < 450*time.Millisecond
}

func (p *popupWindow) onOutsideTimer() {
	if !p.shown {
		return
	}
	state, _, _ := pGetAsyncKeyState.Call(vkLeftButton)
	leftDown := state&0x8000 != 0
	pressed := leftDown && !p.prevLeft
	p.prevLeft = leftDown
	if !pressed || p.stillWarm() {
		return
	}
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	var wnd rect
	pGetWindowRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&wnd)))
	if pointInRect(pt, wnd) {
		return
	}
	// Clic en nuestro icono: lo cierra el toggle de la bandeja.
	// Caché + margen amplio: si el timer cierra y luego llega NIN_SELECT, parece que no hace nada.
	if p.iconCacheOK {
		icon := p.iconCache
		pad := int32(24)
		icon.Left -= pad
		icon.Top -= pad
		icon.Right += pad
		icon.Bottom += pad
		if pointInRect(pt, icon) {
			return
		}
	}
	p.hide()
}

func pointInRect(pt point, rc rect) bool {
	return pt.X >= rc.Left && pt.X < rc.Right && pt.Y >= rc.Top && pt.Y < rc.Bottom
}

func (p *popupWindow) repaint() {
	if !p.visible() {
		return
	}
	p.rebuild()
	w, h := p.layout()
	var rc rect
	pGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&rc)))
	if int(rc.Right) != w || int(rc.Bottom) != h {
		x, y := p.anchorNearTray(w, h)
		pSetWindowPos.Call(uintptr(p.hwnd), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x0010) // NOACTIVATE
	}
	pInvalidateRect.Call(uintptr(p.hwnd), 0, 1)
}

func (p *popupWindow) rebuild() {
	title, detail, online, busy, _ := p.app.snapshot()
	p.actions = []popupAction{
		{label: "Abrir panel", enabled: online, handler: func() { go openPanel("") }},
		{label: "Copiar ahora", enabled: online && !busy, handler: func() {
			go func() {
				if err := panel.RequestBackup(); err != nil {
					pushToast("No se pudo iniciar la copia", err.Error())
					return
				}
				if p.app.hwnd != 0 {
					pPostMessage.Call(uintptr(p.app.hwnd), wmAppRefresh, 0, 0)
				}
			}()
		}},
		{label: "Ver historial", enabled: online, handler: func() { go openPanel("historial") }},
		{label: "Restaurar archivos", enabled: online, handler: func() { go openPanel("restaurar") }},
	}
	_ = title
	_ = detail
}

func (p *popupWindow) layout() (int, int) {
	dpi := 96
	if p.hwnd != 0 && pGetDpiForWindow.Find() == nil {
		if v, _, _ := pGetDpiForWindow.Call(uintptr(p.hwnd)); v != 0 {
			dpi = int(v)
		}
	}
	p.dpi = dpi
	s := func(v int) int { return scale(v, dpi) }

	width := s(320)
	pad := s(12)
	y := pad
	p.rows = p.rows[:0]

	headerH := s(64)
	p.rows = append(p.rows, popupRow{kind: 0, bounds: rect{int32(pad), int32(y), int32(width - pad), int32(y + headerH)}})
	y += headerH + s(8)

	for i := 0; i < 4; i++ {
		h := s(44)
		p.rows = append(p.rows, popupRow{kind: 1, bounds: rect{int32(pad), int32(y), int32(width - pad), int32(y + h)}, action: i})
		y += h
	}
	y += s(10)

	legendH := s(110)
	p.rows = append(p.rows, popupRow{kind: 3, bounds: rect{int32(pad), int32(y), int32(width - pad), int32(y + legendH)}})
	y += legendH + pad
	return width, y
}

func (p *popupWindow) anchorNearTray(w, h int) (int, int) {
	work := primaryWorkArea()
	gap := scale(12, p.dpi)
	// La barra de tareas real (aunque el área de trabajo diga 1440 con ocultar automáticamente).
	if bar, ok := taskbarRect(); ok {
		switch {
		case bar.Top > work.Top+100: // abajo
			if int(bar.Top) < int(work.Bottom) {
				work.Bottom = bar.Top
			}
		case bar.Bottom < work.Bottom-100: // arriba
			if int(bar.Bottom) > int(work.Top) {
				work.Top = bar.Bottom
			}
		case bar.Left > work.Left+100: // derecha
			if int(bar.Left) < int(work.Right) {
				work.Right = bar.Left
			}
		case bar.Right < work.Right-100: // izquierda
			if int(bar.Right) > int(work.Left) {
				work.Left = bar.Right
			}
		}
	} else {
		// Reserva mínima por si no se puede leer la barra.
		work.Bottom -= int32(scale(48, p.dpi))
	}

	var icon rect
	var haveIcon bool
	if p.app != nil {
		icon, haveIcon = p.app.iconRect()
	}

	x := int(work.Right) - w - gap
	y := int(work.Bottom) - h - gap
	if haveIcon && iconUsable(icon, work) {
		x = (int(icon.Left)+int(icon.Right))/2 - w/2
		y = int(icon.Top) - h - gap
		if y+h > int(work.Bottom)-gap {
			y = int(work.Bottom) - h - gap
		}
		if y < int(work.Top)+gap {
			y = int(work.Top) + gap
			x = int(icon.Left) - w - gap
		}
	}
	if x < int(work.Left)+gap {
		x = int(work.Left) + gap
	}
	if x+w > int(work.Right)-gap {
		x = int(work.Right) - w - gap
	}
	if y < int(work.Top)+gap {
		y = int(work.Top) + gap
	}
	if y+h > int(work.Bottom)-gap {
		y = int(work.Bottom) - h - gap
	}
	return x, y
}

func primaryWorkArea() rect {
	var rc rect
	const spiGetWorkArea = 0x0030
	ok, _, _ := pSystemParametersInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&rc)), 0)
	if ok != 0 && rc.Right > rc.Left && rc.Bottom > rc.Top {
		return rc
	}
	return rect{0, 0, 1920, 1040}
}

func rectsOverlap(a, b rect) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}

func iconUsable(icon, work rect) bool {
	if icon.Right <= icon.Left || icon.Bottom <= icon.Top {
		return false
	}
	// Sirve para alinear en X aunque el icono esté en el desbordamiento (^).
	cx := (icon.Left + icon.Right) / 2
	return cx >= work.Left-8 && cx <= work.Right+8
}

func workAreaNearTray(app *trayApp) rect {
	var pt point
	if app != nil {
		if icon, ok := app.iconRect(); ok {
			pt.X = (icon.Left + icon.Right) / 2
			pt.Y = (icon.Top + icon.Bottom) / 2
		}
	}
	if pt.X == 0 && pt.Y == 0 {
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	}
	monitor, _, _ := pMonitorFromPoint.Call(uintptr(uint32(pt.X))|uintptr(uint32(pt.Y))<<32, 2)
	type monitorInfo struct {
		Size    uint32
		Monitor rect
		Work    rect
		Flags   uint32
	}
	mi := monitorInfo{}
	mi.Size = uint32(unsafe.Sizeof(mi))
	if monitor != 0 {
		pGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&mi)))
		return mi.Work
	}
	return rect{0, 0, 1920, 1040}
}

func (p *popupWindow) wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case 0x000F: // WM_PAINT
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			p.paint(windows.Handle(hdc))
			pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case 0x0312: // WM_HOTKEY (Escape)
		if wParam == 1 {
			p.hide()
		}
		return 0
	case wmTimer:
		if wParam == popupTimerID {
			p.onOutsideTimer()
		}
		return 0
	case 0x0200: // WM_MOUSEMOVE
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		p.updateHover(x, y)
		return 0
	case 0x0201: // WM_LBUTTONDOWN
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		if pointInRect(point{int32(x), int32(y)}, p.closeBtn) {
			p.hide()
			return 0
		}
		if action := p.actionAt(x, y); action >= 0 && action < len(p.actions) {
			if !p.actions[action].enabled {
				pushToast("No disponible", "El agente no está en marcha o la acción no se puede usar ahora.")
				return 0
			}
			handler := p.actions[action].handler
			p.hide()
			if handler != nil {
				handler()
			}
		}
		return 0
	case 0x0010:
		p.hide()
		return 0
	}
	ret, _, _ := pDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func (p *popupWindow) hit(x, y int) bool {
	var rc rect
	pGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&rc)))
	return x >= 0 && y >= 0 && x < int(rc.Right) && y < int(rc.Bottom)
}

func (p *popupWindow) actionAt(x, y int) int {
	for _, row := range p.rows {
		if row.kind != 1 {
			continue
		}
		if x >= int(row.bounds.Left) && x < int(row.bounds.Right) && y >= int(row.bounds.Top) && y < int(row.bounds.Bottom) {
			return row.action
		}
	}
	return -1
}

func (p *popupWindow) updateHover(x, y int) {
	next := p.actionAt(x, y)
	if next == p.hover {
		return
	}
	p.hover = next
	pInvalidateRect.Call(uintptr(p.hwnd), 0, 1)
}

func (p *popupWindow) paint(hdc windows.Handle) {
	var rc rect
	pGetClientRect.Call(uintptr(p.hwnd), uintptr(unsafe.Pointer(&rc)))
	bg := brush(rgb(245, 246, 248))
	pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(bg))
	pDeleteObject.Call(uintptr(bg))

	title, detail, online, _, level := p.app.snapshot()
	var card rect
	var hasCard bool
	for _, row := range p.rows {
		if row.kind == 0 || row.kind == 1 || row.kind == 2 {
			if !hasCard {
				card = row.bounds
				hasCard = true
			} else {
				if row.bounds.Left < card.Left {
					card.Left = row.bounds.Left
				}
				if row.bounds.Top < card.Top {
					card.Top = row.bounds.Top
				}
				if row.bounds.Right > card.Right {
					card.Right = row.bounds.Right
				}
				if row.bounds.Bottom > card.Bottom {
					card.Bottom = row.bounds.Bottom
				}
			}
		}
	}
	if hasCard {
		drawCard(hdc, card, scale(16, p.dpi))
	}
	for _, row := range p.rows {
		switch row.kind {
		case 0:
			p.drawHeader(hdc, row.bounds, title, detail, level, online)
		case 1:
			p.drawAction(hdc, row.bounds, row.action, row.action == p.hover)
		case 3:
			drawCard(hdc, row.bounds, scale(16, p.dpi))
			p.drawLegend(hdc, row.bounds)
		}
	}
}

func drawCard(hdc windows.Handle, r rect, rad int) {
	card := brush(0xFFFFFF)
	pen := createPen(rgb(230, 232, 236))
	oldPen, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(pen))
	oldBrush, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(card))
	pRoundRect.Call(uintptr(hdc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(rad), uintptr(rad))
	pSelectObject.Call(uintptr(hdc), oldBrush)
	pSelectObject.Call(uintptr(hdc), oldPen)
	pDeleteObject.Call(uintptr(card))
	pDeleteObject.Call(uintptr(pen))
}

func (p *popupWindow) drawHeader(hdc windows.Handle, r rect, title, detail, level string, online bool) {
	soft := headerColor(level, online)
	softBrush := brush(soft)
	rad := int32(scale(16, p.dpi))
	nullPen := createPen(soft)
	oldPen, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(nullPen))
	oldBrush, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(softBrush))
	inner := rect{r.Left + 1, r.Top + 1, r.Right - 1, r.Bottom}
	pRoundRect.Call(uintptr(hdc), uintptr(inner.Left), uintptr(inner.Top), uintptr(inner.Right), uintptr(inner.Bottom), uintptr(rad), uintptr(rad))
	cover := rect{inner.Left, r.Bottom - int32(scale(18, p.dpi)), inner.Right, r.Bottom}
	pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&cover)), uintptr(softBrush))
	pSelectObject.Call(uintptr(hdc), oldBrush)
	pSelectObject.Call(uintptr(hdc), oldPen)
	pDeleteObject.Call(uintptr(softBrush))
	pDeleteObject.Call(uintptr(nullPen))

	cx := int(r.Left) + scale(28, p.dpi)
	cy := int(r.Top) + scale(32, p.dpi)
	drawStatusDot(hdc, cx, cy, scale(14, p.dpi), levelColor(level, online))

	// Botón cerrar: así no hace falta volver a acertar el icono de la bandeja.
	btn := scale(28, p.dpi)
	p.closeBtn = rect{
		r.Right - int32(scale(12, p.dpi)) - int32(btn),
		r.Top + int32(scale(10, p.dpi)),
		r.Right - int32(scale(12, p.dpi)),
		r.Top + int32(scale(10, p.dpi)) + int32(btn),
	}
	drawCloseButton(hdc, p.closeBtn)

	pSetBkMode.Call(uintptr(hdc), 1)
	pSetTextColor.Call(uintptr(hdc), uintptr(rgb(17, 24, 39)))
	fontTitle := font(scale(15, p.dpi), true)
	oldFont, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(fontTitle))
	textRect := rect{r.Left + int32(scale(52, p.dpi)), r.Top + int32(scale(12, p.dpi)), p.closeBtn.Left - int32(scale(8, p.dpi)), r.Top + int32(scale(34, p.dpi))}
	drawText(hdc, title, textRect, 0x0000)
	pSelectObject.Call(uintptr(hdc), oldFont)
	pDeleteObject.Call(uintptr(fontTitle))

	pSetTextColor.Call(uintptr(hdc), uintptr(rgb(100, 108, 122)))
	fontDetail := font(scale(12, p.dpi), false)
	oldFont, _, _ = pSelectObject.Call(uintptr(hdc), uintptr(fontDetail))
	textRect = rect{r.Left + int32(scale(52, p.dpi)), r.Top + int32(scale(34, p.dpi)), p.closeBtn.Left - int32(scale(8, p.dpi)), r.Top + int32(scale(54, p.dpi))}
	drawText(hdc, detail, textRect, 0x0020)
	pSelectObject.Call(uintptr(hdc), oldFont)
	pDeleteObject.Call(uintptr(fontDetail))
}

func drawCloseButton(hdc windows.Handle, r rect) {
	pen := createPenWidth(rgb(100, 108, 122), 2)
	oldPen, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(pen))
	pad := (r.Right - r.Left) / 4
	pMoveToEx.Call(uintptr(hdc), uintptr(r.Left+pad), uintptr(r.Top+pad), 0)
	pLineTo.Call(uintptr(hdc), uintptr(r.Right-pad), uintptr(r.Bottom-pad))
	pMoveToEx.Call(uintptr(hdc), uintptr(r.Right-pad), uintptr(r.Top+pad), 0)
	pLineTo.Call(uintptr(hdc), uintptr(r.Left+pad), uintptr(r.Bottom-pad))
	pSelectObject.Call(uintptr(hdc), oldPen)
	pDeleteObject.Call(uintptr(pen))
}

func (p *popupWindow) drawAction(hdc windows.Handle, r rect, action int, hover bool) {
	if action < 0 || action >= len(p.actions) {
		return
	}
	item := p.actions[action]
	if hover && item.enabled {
		b := brush(rgb(235, 240, 252))
		pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&r)), uintptr(b))
		pDeleteObject.Call(uintptr(b))
	}
	iconColor := rgb(35, 80, 216)
	textColor := rgb(17, 24, 39)
	if !item.enabled {
		iconColor = rgb(160, 166, 176)
		textColor = rgb(160, 166, 176)
	}
	cx := int(r.Left) + scale(22, p.dpi)
	cy := (int(r.Top) + int(r.Bottom)) / 2
	drawActionIcon(hdc, action, cx, cy, scale(8, p.dpi), iconColor)

	pSetBkMode.Call(uintptr(hdc), 1)
	pSetTextColor.Call(uintptr(hdc), uintptr(textColor))
	weight := action == 0
	f := font(scale(14, p.dpi), weight)
	old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(f))
	textRect := rect{r.Left + int32(scale(44, p.dpi)), r.Top, r.Right - int32(scale(12, p.dpi)), r.Bottom}
	drawText(hdc, item.label, textRect, 0x0024) // left | vcenter
	pSelectObject.Call(uintptr(hdc), old)
	pDeleteObject.Call(uintptr(f))
}

func (p *popupWindow) drawLegend(hdc windows.Handle, r rect) {
	pSetBkMode.Call(uintptr(hdc), 1)
	pSetTextColor.Call(uintptr(hdc), uintptr(rgb(100, 108, 122)))
	f := font(scale(12, p.dpi), true)
	old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(f))
	title := rect{r.Left + int32(scale(14, p.dpi)), r.Top + int32(scale(10, p.dpi)), r.Right - int32(scale(14, p.dpi)), r.Top + int32(scale(28, p.dpi))}
	drawText(hdc, "Colores del icono", title, 0)
	pSelectObject.Call(uintptr(hdc), old)
	pDeleteObject.Call(uintptr(f))

	items := []struct {
		label string
		color uint32
	}{
		{"Al día", rgb(23, 121, 74)},
		{"Con avisos", rgb(224, 161, 0)},
		{"Falla o sin copia", rgb(168, 36, 26)},
		{"Copiando", rgb(35, 80, 216)},
		{"Servicio parado", rgb(138, 144, 155)},
	}
	f2 := font(scale(12, p.dpi), false)
	old, _, _ = pSelectObject.Call(uintptr(hdc), uintptr(f2))
	pSetTextColor.Call(uintptr(hdc), uintptr(rgb(55, 65, 81)))
	colW := (int(r.Right-r.Left) - scale(28, p.dpi)) / 2
	for i, item := range items {
		col := i % 2
		row := i / 2
		x := int(r.Left) + scale(14, p.dpi) + col*colW
		y := int(r.Top) + scale(36, p.dpi) + row*scale(24, p.dpi)
		drawStatusDot(hdc, x+scale(6, p.dpi), y+scale(8, p.dpi), scale(6, p.dpi), item.color)
		tr := rect{int32(x + scale(18, p.dpi)), int32(y), int32(x + colW - scale(8, p.dpi)), int32(y + scale(18, p.dpi))}
		drawText(hdc, item.label, tr, 0x0024)
	}
	pSelectObject.Call(uintptr(hdc), old)
	pDeleteObject.Call(uintptr(f2))
}

func headerColor(level string, online bool) uint32 {
	if !online {
		return rgb(238, 240, 243)
	}
	switch level {
	case status.Green:
		return rgb(230, 245, 236)
	case status.Yellow:
		return rgb(255, 246, 220)
	case status.Red:
		return rgb(252, 232, 230)
	case status.Blue:
		return rgb(232, 238, 252)
	default:
		return rgb(238, 240, 243)
	}
}

func levelColor(level string, online bool) uint32 {
	if !online {
		return rgb(138, 144, 155)
	}
	switch level {
	case status.Green:
		return rgb(23, 121, 74)
	case status.Yellow:
		return rgb(224, 161, 0)
	case status.Red:
		return rgb(168, 36, 26)
	case status.Blue:
		return rgb(35, 80, 216)
	default:
		return rgb(138, 144, 155)
	}
}

func drawStatusDot(hdc windows.Handle, cx, cy, radius int, color uint32) {
	b := brush(color)
	pen := createPen(color)
	oldB, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(b))
	oldP, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(pen))
	pEllipse.Call(uintptr(hdc), uintptr(cx-radius), uintptr(cy-radius), uintptr(cx+radius), uintptr(cy+radius))
	pSelectObject.Call(uintptr(hdc), oldB)
	pSelectObject.Call(uintptr(hdc), oldP)
	pDeleteObject.Call(uintptr(b))
	pDeleteObject.Call(uintptr(pen))
}

func drawActionIcon(hdc windows.Handle, action, cx, cy, r int, color uint32) {
	pen := createPenWidth(color, 2)
	oldP, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(pen))
	nullBrush, _, _ := pGetStockObject.Call(5) // NULL_BRUSH
	oldB, _, _ := pSelectObject.Call(uintptr(hdc), nullBrush)
	switch action {
	case 0: // panel
		pRoundRect.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy-r), uintptr(cx+r), uintptr(cy+r), 2, 2)
		pMoveToEx.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy-r/2), 0)
		pLineTo.Call(uintptr(hdc), uintptr(cx+r), uintptr(cy-r/2))
	case 1: // upload
		pMoveToEx.Call(uintptr(hdc), uintptr(cx), uintptr(cy+r), 0)
		pLineTo.Call(uintptr(hdc), uintptr(cx), uintptr(cy-r))
		pMoveToEx.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy-r/3), 0)
		pLineTo.Call(uintptr(hdc), uintptr(cx), uintptr(cy-r))
		pLineTo.Call(uintptr(hdc), uintptr(cx+r), uintptr(cy-r/3))
	case 2: // history
		pMoveToEx.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy-r/2), 0)
		pLineTo.Call(uintptr(hdc), uintptr(cx+r), uintptr(cy-r/2))
		pMoveToEx.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy), 0)
		pLineTo.Call(uintptr(hdc), uintptr(cx+r), uintptr(cy))
		pMoveToEx.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy+r/2), 0)
		pLineTo.Call(uintptr(hdc), uintptr(cx+r), uintptr(cy+r/2))
	case 3: // restore
		pEllipse.Call(uintptr(hdc), uintptr(cx-r), uintptr(cy-r), uintptr(cx+r), uintptr(cy+r))
	}
	pSelectObject.Call(uintptr(hdc), oldB)
	pSelectObject.Call(uintptr(hdc), oldP)
	pDeleteObject.Call(uintptr(pen))
}

func brush(color uint32) windows.Handle {
	h, _, _ := pCreateSolidBrush.Call(uintptr(color))
	return windows.Handle(h)
}

func createPen(color uint32) windows.Handle {
	return createPenWidth(color, 1)
}

func createPenWidth(color uint32, width int) windows.Handle {
	h, _, _ := pCreatePen.Call(0, uintptr(width), uintptr(color))
	return windows.Handle(h)
}

func font(size int, bold bool) windows.Handle {
	weight := uintptr(400)
	if bold {
		weight = 700
	}
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	h, _, _ := pCreateFont.Call(
		uintptr(size), 0, 0, 0, weight, 0, 0, 0,
		1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)),
	)
	return windows.Handle(h)
}

func drawText(hdc windows.Handle, text string, r rect, format uint32) {
	ptr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	rc := r
	pDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(ptr)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), uintptr(format|0x00000020))
}
