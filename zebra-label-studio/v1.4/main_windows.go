//go:build windows

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	winspool = syscall.NewLazyDLL("winspool.drv")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW    = user32.NewProc("RegisterClassExW")
	pCreateWindowExW     = user32.NewProc("CreateWindowExW")
	pDefWindowProcW      = user32.NewProc("DefWindowProcW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pUpdateWindow        = user32.NewProc("UpdateWindow")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessageW    = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pPostMessageW        = user32.NewProc("PostMessageW")
	pSetWindowTextW      = user32.NewProc("SetWindowTextW")
	pGetWindowTextW      = user32.NewProc("GetWindowTextW")
	pGetWindowTextLen    = user32.NewProc("GetWindowTextLengthW")
	pSendMessageW        = user32.NewProc("SendMessageW")
	pMessageBoxW         = user32.NewProc("MessageBoxW")
	pGetDC               = user32.NewProc("GetDC")
	pReleaseDC           = user32.NewProc("ReleaseDC")
	pInvalidateRect      = user32.NewProc("InvalidateRect")
	pBeginPaint          = user32.NewProc("BeginPaint")
	pEndPaint            = user32.NewProc("EndPaint")
	pGetClientRect       = user32.NewProc("GetClientRect")
	pSetCapture          = user32.NewProc("SetCapture")
	pReleaseCapture      = user32.NewProc("ReleaseCapture")
	pSetFocus            = user32.NewProc("SetFocus")
	pFindWindowW         = user32.NewProc("FindWindowW")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pBringWindowToTop    = user32.NewProc("BringWindowToTop")
	pSetWindowPos        = user32.NewProc("SetWindowPos")

	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
	pFillRect         = user32.NewProc("FillRect")
	pRectangle        = gdi32.NewProc("Rectangle")
	pMoveToEx         = gdi32.NewProc("MoveToEx")
	pLineTo           = gdi32.NewProc("LineTo")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pTextOutW         = gdi32.NewProc("TextOutW")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pCreateFontW      = gdi32.NewProc("CreateFontW")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")

	pEnumPrintersW    = winspool.NewProc("EnumPrintersW")
	pOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	pClosePrinter     = winspool.NewProc("ClosePrinter")
	pStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	pEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	pStartPagePrinter = winspool.NewProc("StartPagePrinter")
	pEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	pWritePrinter     = winspool.NewProc("WritePrinter")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
)

const (
	WS_OVERLAPPEDWINDOW      = 0x00CF0000
	WS_CHILD                 = 0x40000000
	WS_VISIBLE               = 0x10000000
	WS_BORDER                = 0x00800000
	WS_TABSTOP               = 0x00010000
	BS_PUSHBUTTON            = 0x00000000
	BS_AUTOCHECKBOX          = 0x00000003
	CBS_DROPDOWN             = 0x0002
	CBS_DROPDOWNLIST         = 0x0003
	ES_AUTOHSCROLL           = 0x0080
	LBS_NOTIFY               = 0x0001
	SW_SHOW                  = 5
	SW_RESTORE               = 9
	HWND_TOP                 = 0
	SWP_NOSIZE               = 0x0001
	SWP_NOMOVE               = 0x0002
	SWP_SHOWWINDOW           = 0x0040
	WM_DESTROY               = 0x0002
	WM_COMMAND               = 0x0111
	WM_PAINT                 = 0x000F
	WM_LBUTTONDOWN           = 0x0201
	WM_LBUTTONUP             = 0x0202
	WM_MOUSEMOVE             = 0x0200
	WM_SIZE                  = 0x0005
	WM_APP                   = 0x8000
	WM_PDF_EVENT             = WM_APP + 21
	WM_PRINT_EVENT           = WM_APP + 22
	WM_PRINTER_EVENT         = WM_APP + 23
	WM_CSV_EVENT             = WM_APP + 24
	BM_GETCHECK              = 0x00F0
	BM_SETCHECK              = 0x00F1
	BST_CHECKED              = 1
	CB_ADDSTRING             = 0x0143
	CB_GETCURSEL             = 0x0147
	CB_RESETCONTENT          = 0x014B
	CB_SETCURSEL             = 0x014E
	CBN_SELCHANGE            = 1
	BN_CLICKED               = 0
	EN_CHANGE                = 0x0300
	LB_ADDSTRING             = 0x0180
	LB_RESETCONTENT          = 0x0184
	MB_OK                    = 0x00000000
	MB_ICONERROR             = 0x00000010
	MB_ICONINFORMATION       = 0x00000040
	OFN_FILEMUSTEXIST        = 0x00001000
	OFN_PATHMUSTEXIST        = 0x00000800
	TRANSPARENT              = 1
	PRINTER_ENUM_LOCAL       = 0x00000002
	PRINTER_ENUM_CONNECTIONS = 0x00000004
)

const (
	ID_PRINTER      = 101
	ID_REFRESH      = 102
	ID_DPI          = 103
	ID_LABEL_W      = 104
	ID_LABEL_H      = 105
	ID_MEDIA_W      = 106
	ID_GAP          = 107
	ID_DM_SIZE      = 108
	ID_X            = 109
	ID_Y            = 110
	ID_OPEN_CSV     = 111
	ID_OPEN_PDF     = 112
	ID_PRINT_ONE    = 113
	ID_PRINT_ALL    = 114
	ID_TEST         = 115
	ID_TEXT1_CHECK  = 121
	ID_TEXT1        = 122
	ID_TEXT2_CHECK  = 123
	ID_TEXT2        = 124
	ID_TEXT3_CHECK  = 125
	ID_TEXT3        = 126
	ID_TEXT_SIZE    = 127
	ID_LABEL_OFFSET = 128
	ID_CENTER_LABEL = 129
	ID_EXPORT_ZPL   = 130
	ID_REPRINT_N    = 131
	ID_REPRINT_LAST = 132
	ID_FIND_PRINTER = 133
	ID_STATUS       = 140
	ID_CODELIST     = 141
	ID_QUEUE_STATUS = 142
)

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}
type POINT struct{ X, Y int32 }
type MSG struct {
	HWnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	Hdc         syscall.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type OPENFILENAME struct {
	lStructSize       uint32
	hwndOwner         syscall.Handle
	hInstance         syscall.Handle
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	Flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}
type DOC_INFO_1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}
type PRINTER_INFO_4 struct {
	pPrinterName *uint16
	pServerName  *uint16
	Attributes   uint32
}

type Element struct {
	kind       int
	x, y, w, h float64
}

var (
	hMain, hCanvas      syscall.Handle
	controls            = map[int]syscall.Handle{}
	codes               [][]byte
	currentFile         string
	currentPDF          string
	codeSources         []string
	pdfBusy             atomic.Bool
	selected            = 0
	dragging            bool
	dragDX, dragDY      float64
	suppressControlSync bool
	printerScanRunning  atomic.Bool
	printerMu           sync.Mutex
	printerFound        string
	elems               = []Element{{kind: 0, x: 2, y: 2, w: 11, h: 11}, {kind: 1, x: 2, y: 15, w: 20, h: 3}, {kind: 2, x: 2, y: 19, w: 20, h: 3}, {kind: 3, x: 2, y: 23, w: 20, h: 3}}
)

func ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func utf16String(p *uint16) string {
	if p == nil {
		return ""
	}
	a := []uint16{}
	for i := uintptr(0); ; i++ {
		v := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + i*2))
		if v == 0 {
			break
		}
		a = append(a, v)
	}
	return string(utf16.Decode(a))
}
func loword(v uintptr) int { return int(v & 0xffff) }
func hiword(v uintptr) int { return int((v >> 16) & 0xffff) }

type pdfWorkerResult struct {
	Codes   [][]byte `json:"codes"`
	Sources []string `json:"sources"`
	Error   string   `json:"error,omitempty"`
}

func runPDFHelperMode() bool {
	if len(os.Args) < 4 || os.Args[1] != "--pdf-helper" {
		return false
	}
	pdfPath := os.Args[2]
	outPath := os.Args[3]
	res := pdfWorkerResult{}
	defer func() {
		if r := recover(); r != nil {
			res.Error = fmt.Sprintf("внутренняя ошибка PDF-сканера: %v", r)
		}
		b, _ := json.Marshal(res)
		_ = os.WriteFile(outPath, b, 0600)
	}()
	codes, sources, err := scanPDFCodes(pdfPath, func(string) {})
	if err != nil {
		res.Error = err.Error()
		return true
	}
	res.Codes = codes
	res.Sources = sources
	return true
}

func logCrash(where string, v any) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "MSK", "Zebra Label Studio")
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "error.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		defer f.Close()
		fmt.Fprintf(f, "%s: %v\r\n", where, v)
	}
}

func crashGuard(where string) {
	if r := recover(); r != nil {
		logCrash(where, r)
		msgbox(fmt.Sprintf("Внутренняя ошибка %s: %v\n\nЛог: %%LOCALAPPDATA%%\\MSK\\Zebra Label Studio\\error.log", where, r), true)
	}
}

func main() {
	defer crashGuard("main")
	// PDF decoding runs in a separate process. A native decoder crash must never
	// be able to terminate the main GUI process.
	if runPDFHelperMode() {
		return
	}
	className := ptr("MSK_ZebraLabelStudio")
	// Single-instance behavior: if a previous copy is already running,
	// restore and bring its window to the foreground instead of starting
	// an invisible duplicate process.
	if existing, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0); existing != 0 {
		pShowWindow.Call(existing, SW_RESTORE)
		pBringWindowToTop.Call(existing)
		pSetWindowPos.Call(existing, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
		pSetForegroundWindow.Call(existing)
		return
	}
	hInst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{cbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), lpfnWndProc: syscall.NewCallback(wndProc), hInstance: syscall.Handle(hInst), hbrBackground: syscall.Handle(6), lpszClassName: className}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	h, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(ptr("MSK Zebra Label Studio v1.4 — Честный Знак"))), WS_OVERLAPPEDWINDOW, 80, 50, 1180, 760, 0, 0, hInst, 0)
	hMain = syscall.Handle(h)
	buildUI(hMain, syscall.Handle(hInst))
	pShowWindow.Call(h, SW_SHOW)
	pUpdateWindow.Call(h)
	// Make the main window visible even when launched by an installer or
	// from a stale shortcut. This avoids the "process is running but I
	// cannot see the program" situation.
	pShowWindow.Call(h, SW_RESTORE)
	pBringWindowToTop.Call(h)
	pSetWindowPos.Call(h, HWND_TOP, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
	pSetForegroundWindow.Call(h)
	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func create(class, text string, style uint32, x, y, w, h int, parent syscall.Handle, id int) syscall.Handle {
	r, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(ptr(class))), uintptr(unsafe.Pointer(ptr(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	hw := syscall.Handle(r)
	if id != 0 {
		controls[id] = hw
	}
	return hw
}
func label(text string, x, y, w int, parent syscall.Handle) {
	create("STATIC", text, WS_CHILD|WS_VISIBLE, x, y, w, 20, parent, 0)
}

type pdfUIEvent struct {
	kind    string
	status  string
	found   [][]byte
	sources []string
	err     error
}

var (
	pdfEventMu  sync.Mutex
	pdfEvents   = map[uintptr]pdfUIEvent{}
	pdfEventSeq atomic.Uint64
)

func postPDFEvent(ev pdfUIEvent) {
	id := uintptr(pdfEventSeq.Add(1))
	pdfEventMu.Lock()
	pdfEvents[id] = ev
	pdfEventMu.Unlock()
	pPostMessageW.Call(uintptr(hMain), WM_PDF_EVENT, id, 0)
}

func takePDFEvent(id uintptr) (pdfUIEvent, bool) {
	pdfEventMu.Lock()
	defer pdfEventMu.Unlock()
	ev, ok := pdfEvents[id]
	if ok {
		delete(pdfEvents, id)
	}
	return ev, ok
}

func buildUI(parent syscall.Handle, hInst syscall.Handle) {
	label("Принтер Zebra / ZDesigner", 18, 18, 165, parent)
	create("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|CBS_DROPDOWN, 185, 14, 260, 220, parent, ID_PRINTER)
	create("BUTTON", "Найти Zebra", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 453, 14, 105, 26, parent, ID_FIND_PRINTER)
	label("DPI", 570, 18, 35, parent)
	create("EDIT", "203", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 605, 14, 60, 26, parent, ID_DPI)
	label("Наклейка, мм", 18, 58, 100, parent)
	label("Ш", 120, 58, 18, parent)
	create("EDIT", "15", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 140, 54, 55, 24, parent, ID_LABEL_W)
	label("В", 205, 58, 18, parent)
	create("EDIT", "15", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 225, 54, 55, 24, parent, ID_LABEL_H)
	label("Физ. ширина ленты", 300, 58, 120, parent)
	create("EDIT", "26", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 425, 54, 55, 24, parent, ID_MEDIA_W)
	label("Зазор", 495, 58, 45, parent)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 540, 54, 55, 24, parent, ID_GAP)
	label("Смещение этикетки X", 610, 58, 140, parent)
	create("EDIT", "0", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 755, 54, 55, 24, parent, ID_LABEL_OFFSET)
	center := create("BUTTON", "Центрировать на ленте", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 825, 54, 175, 24, parent, ID_CENTER_LABEL)
	pSendMessageW.Call(uintptr(center), BM_SETCHECK, BST_CHECKED, 0)
	label("Data Matrix, мм", 18, 96, 110, parent)
	create("EDIT", "11", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 130, 92, 55, 24, parent, ID_DM_SIZE)
	label("X", 200, 96, 15, parent)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 218, 92, 55, 24, parent, ID_X)
	label("Y", 285, 96, 15, parent)
	create("EDIT", "2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 303, 92, 55, 24, parent, ID_Y)
	label("Размер шрифта, мм", 375, 96, 120, parent)
	create("EDIT", "2.2", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 500, 92, 55, 24, parent, ID_TEXT_SIZE)
	create("BUTTON", "GTIN: {GTIN}", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 18, 132, 125, 22, parent, ID_TEXT1_CHECK)
	create("EDIT", "GTIN: {GTIN}", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 150, 130, 405, 24, parent, ID_TEXT1)
	create("BUTTON", "Серийный", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 18, 162, 125, 22, parent, ID_TEXT2_CHECK)
	create("EDIT", "SN: {SERIAL}", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 150, 160, 405, 24, parent, ID_TEXT2)
	create("BUTTON", "Текст", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 18, 192, 125, 22, parent, ID_TEXT3_CHECK)
	create("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 150, 190, 405, 24, parent, ID_TEXT3)
	create("BUTTON", "Открыть CSV", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 18, 230, 110, 30, parent, ID_OPEN_CSV)
	create("BUTTON", "Открыть PDF", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 138, 230, 110, 30, parent, ID_OPEN_PDF)
	create("BUTTON", "Сохранить ZPL", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 258, 230, 110, 30, parent, ID_EXPORT_ZPL)
	create("BUTTON", "Тест этикетки", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 378, 230, 110, 30, parent, ID_TEST)
	create("BUTTON", "Печать 1", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 498, 230, 85, 30, parent, ID_PRINT_ONE)
	create("BUTTON", "Печать всех", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 18, 267, 110, 28, parent, ID_PRINT_ALL)
	label("Перепечатать последние", 145, 272, 145, parent)
	create("EDIT", "1", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_AUTOHSCROLL, 292, 267, 45, 28, parent, ID_REPRINT_N)
	create("BUTTON", "Перепечатать", WS_CHILD|WS_VISIBLE|BS_PUSHBUTTON, 345, 267, 110, 28, parent, ID_REPRINT_LAST)
	create("STATIC", "Очередь: партия не загружена", WS_CHILD|WS_VISIBLE, 610, 625, 520, 22, parent, ID_QUEUE_STATUS)
	create("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|LBS_NOTIFY, 18, 305, 555, 330, parent, ID_CODELIST)
	label("Макет этикетки — Data Matrix и текст можно двигать мышью по X/Y", 610, 18, 500, parent)
	canvasClass := ptr("MSK_LabelCanvas")
	wc := WNDCLASSEX{cbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), lpfnWndProc: syscall.NewCallback(canvasProc), hInstance: hInst, hbrBackground: syscall.Handle(6), lpszClassName: canvasClass}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	r, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(canvasClass)), 0, WS_CHILD|WS_VISIBLE|WS_BORDER, 610, 45, 520, 590, uintptr(parent), 0, uintptr(hInst), 0)
	hCanvas = syscall.Handle(r)
	st := create("STATIC", "Готово. CSV/PDF и макет работают без принтера. Для печати укажите точное имя очереди Zebra / ZDesigner на рабочем ПК.", WS_CHILD|WS_VISIBLE, 18, 650, 1110, 30, parent, ID_STATUS)
	_ = st
}

func wndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logCrash("wndProc", r)
			ret = 0
		}
	}()
	switch msg {
	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	case WM_PRINTER_EVENT:
		if ev, ok := takePrinterEvent(wparam); ok {
			printerScanRunning.Store(false)
			if ev.err != nil {
				setStatus("Поиск Zebra: " + ev.err.Error())
				msgbox("Не удалось получить список принтеров:\n\n"+ev.err.Error()+"\n\nОсновная программа продолжает работать. Имя Zebra можно ввести вручную.", true)
				return 0
			}
			if len(ev.names) == 0 {
				setStatus("Zebra / ZDesigner не найдена среди установленных очередей Windows.")
				msgbox("Zebra / ZDesigner не найдена.\n\nПроверьте, что драйвер принтера установлен в Windows и принтер виден в 'Принтеры и сканеры'.", false)
				return 0
			}
			// Заполняем выпадающий список всеми найденными очередями Zebra/ZDesigner.
			hp := controls[ID_PRINTER]
			pSendMessageW.Call(uintptr(hp), CB_RESETCONTENT, 0, 0)
			for _, name := range ev.names {
				pSendMessageW.Call(uintptr(hp), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(ptr(name))))
			}
			pSendMessageW.Call(uintptr(hp), CB_SETCURSEL, 0, 0)
			setText(ID_PRINTER, ev.names[0])
			if len(ev.names) == 1 {
				setStatus("Найдена Zebra: " + ev.names[0])
			} else {
				setStatus(fmt.Sprintf("Найдено Zebra/ZDesigner: %d. Выберите нужную очередь в списке принтеров.", len(ev.names)))
			}
		}
		return 0
	case WM_PDF_EVENT:
		if ev, ok := takePDFEvent(wparam); ok {
			switch ev.kind {
			case "progress":
				setStatus(ev.status)
			case "done":
				pdfBusy.Store(false)
				if ev.err != nil {
					setStatus("Ошибка распознавания PDF: " + ev.err.Error())
					msgbox("Не удалось распознать PDF:\n\n"+ev.err.Error(), true)
					return 0
				}
				codeSources = ev.sources
				codes = ev.found
				activateBatchJournal(currentPDF)
				refreshCodeList()
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
				if len(ev.found) == 0 {
					setStatus("PDF обработан, но Data Matrix не найден. Попробуйте другой PDF или более качественный исходник.")
				} else {
					setStatus(fmt.Sprintf("PDF: %s — найдено полных кодов: %d. Готово к печати.", filepath.Base(currentPDF), len(ev.found)))
				}
			}
		}
		return 0
	case WM_PRINT_EVENT:
		if ev, ok := takePrintEvent(wparam); ok {
			printBusy.Store(false)
			confirmed := readProgressIndices(ev.progressPath)
			if len(confirmed) > 0 {
				applyConfirmedPrints(confirmed, ev.isReprint)
			}
			if ev.progressPath != "" && ev.isReprint {
				_ = os.Remove(ev.progressPath)
			}
			if ev.err != nil {
				setStatus(fmt.Sprintf("Печать остановлена: подтверждено %d из %d. %s", len(confirmed), len(ev.requested), ev.err.Error()))
				msgbox(fmt.Sprintf("Печать остановлена.\n\nПодтверждено передачей в очередь Zebra: %d из %d.\nСледующая обычная печать продолжит с первого неподтверждённого кода.\n\n%s", len(confirmed), len(ev.requested), ev.err.Error()), true)
			} else {
				setStatus(ev.status)
			}
			updateQueueStatus()
		}
		return 0
	case WM_CSV_EVENT:
		if ev, ok := takeCSVEvent(wparam); ok {
			csvBusy.Store(false)
			if ev.err != nil {
				setStatus("Ошибка CSV: " + ev.err.Error())
				msgbox("Ошибка чтения CSV:\n\n"+ev.err.Error(), true)
				return 0
			}
			codes = ev.codes
			codeSources = make([]string, len(ev.codes))
			for i := range codeSources {
				codeSources[i] = "CSV"
			}
			currentFile = ev.path
			currentPDF = ""
			activateBatchJournal(ev.path)
			setStatus(fmt.Sprintf("CSV: %s — загружено кодов: %d", filepath.Base(ev.path), len(codes)))
			refreshCodeList()
			pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
		}
		return 0
	case WM_COMMAND:
		id := loword(wparam)
		notify := hiword(wparam)
		// IMPORTANT: route by notification code. A ComboBox selection is not a
		// button click and an Edit EN_CHANGE is not a generic command. Keeping
		// these paths separate prevents re-entrant Win32 processing/freezes.
		switch id {
		case ID_PRINTER:
			// ComboBox локальный: выбор/ввод только меняет текст. Никаких обращений к spooler.
			// Найденные Zebra добавляются кнопкой «Найти Zebra».
		case ID_FIND_PRINTER:
			if notify == BN_CLICKED {
				startPrinterScan()
			}
		case ID_DPI:
			// DPI is a plain local value. No driver/spooler calls and no synchronous
			// repaint while the user is typing; the next paint/print reads it.
			if notify == EN_CHANGE {
			}
		case ID_OPEN_CSV:
			if notify == BN_CLICKED {
				if f := openFile(hwnd, []string{"CSV / TXT", "*.csv;*.txt", "Все файлы", "*.*"}); f != "" {
					loadCSVAsync(f)
				}
			}
		case ID_OPEN_PDF:
			if notify == BN_CLICKED {
				if f := openFile(hwnd, []string{"PDF", "*.pdf"}); f != "" {
					loadPDFAsync(f)
				}
			}
		case ID_EXPORT_ZPL:
			if notify == BN_CLICKED {
				exportZPL()
			}
		case ID_TEST:
			if notify == BN_CLICKED {
				printTest()
			}
		case ID_CENTER_LABEL:
			if notify == BN_CLICKED {
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
			}
		case ID_PRINT_ONE:
			if notify == BN_CLICKED {
				printOne()
			}
		case ID_PRINT_ALL:
			if notify == BN_CLICKED {
				printAll()
			}
		case ID_REPRINT_LAST:
			if notify == BN_CLICKED {
				reprintLast()
			}
		case ID_LABEL_W, ID_LABEL_H, ID_MEDIA_W, ID_GAP, ID_DM_SIZE, ID_X, ID_Y, ID_TEXT_SIZE, ID_LABEL_OFFSET:
			if notify == EN_CHANGE && !suppressControlSync {
				syncFromControls()
				pInvalidateRect.Call(uintptr(hCanvas), 0, 1)
			}
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func canvasProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logCrash("canvasProc", r)
			ret = 0
		}
	}()
	switch msg {
	case WM_PAINT:
		paintCanvas(hwnd)
		return 0
	case WM_LBUTTONDOWN:
		x := int16(loword(lparam))
		y := int16(hiword(lparam))
		selected = hitElement(float64(x), float64(y))
		if selected >= 0 {
			dragging = true
			sx, sy := canvasToMM(float64(x), float64(y))
			dragDX = sx - elems[selected].x
			dragDY = sy - elems[selected].y
			pSetCapture.Call(uintptr(hwnd))
			pSetFocus.Call(uintptr(hwnd))
		}
		return 0
	case WM_MOUSEMOVE:
		if dragging && selected >= 0 {
			x := int16(loword(lparam))
			y := int16(hiword(lparam))
			mx, my := canvasToMM(float64(x), float64(y))
			e := &elems[selected]
			lw := getFloat(ID_LABEL_W, 15)
			lh := getFloat(ID_LABEL_H, 15)
			maxX := maxf(0, lw-e.w)
			maxY := maxf(0, lh-e.h)
			e.x = minf(maxX, maxf(0, mx-dragDX))
			e.y = minf(maxY, maxf(0, my-dragDY))
			if selected == 0 {
				// Обновляем X и Y как одну операцию: EN_CHANGE не должен сбрасывать вторую координату.
				suppressControlSync = true
				setEditFloat(ID_X, e.x)
				setEditFloat(ID_Y, e.y)
				suppressControlSync = false
			}
			pInvalidateRect.Call(uintptr(hwnd), 0, 1)
		}
		return 0
	case WM_LBUTTONUP:
		if dragging {
			dragging = false
			pReleaseCapture.Call()
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func paintCanvas(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	var rc RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	white, _, _ := pCreateSolidBrush.Call(0x00FFFFFF)
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), white)
	pDeleteObject.Call(white)
	lw := getFloat(ID_LABEL_W, 15)
	lh := getFloat(ID_LABEL_H, 15)
	if lw <= 0 || lh <= 0 {
		return
	}
	scale := minf(440/lw, 500/lh)
	ox := 40.0
	oy := 40.0
	// sticker boundary
	pRectangle.Call(hdc, uintptr(int(ox)), uintptr(int(oy)), uintptr(int(ox+lw*scale)), uintptr(int(oy+lh*scale)))
	// draw DM mock grid
	e := elems[0]
	x0 := ox + e.x*scale
	y0 := oy + e.y*scale
	sz := e.w * scale
	cell := sz / 18
	black, _, _ := pCreateSolidBrush.Call(0)
	defer pDeleteObject.Call(black)
	for r := 0; r < 18; r++ {
		for c := 0; c < 18; c++ {
			on := c == 0 || r == 17 || ((r+c*3)%5 < 2)
			if r == 0 {
				on = c%2 == 0
			}
			if c == 17 {
				on = r%2 == 0
			}
			if on {
				rr := RECT{int32(x0 + float64(c)*cell), int32(y0 + float64(r)*cell), int32(x0 + float64(c+1)*cell), int32(y0 + float64(r+1)*cell)}
				pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rr)), black)
			}
		}
	}
	// selection border
	if selected == 0 {
		pRectangle.Call(hdc, uintptr(int(x0-2)), uintptr(int(y0-2)), uintptr(int(x0+sz+2)), uintptr(int(y0+sz+2)))
	}
	pSetBkMode.Call(hdc, TRANSPARENT)
	for i := 1; i <= 3; i++ {
		if !textEnabled(i) {
			continue
		}
		tx := ox + elems[i].x*scale
		ty := oy + elems[i].y*scale
		text := getText(ID_TEXT1 + (i-1)*2)
		if text == "" {
			text = "Текст"
		}
		pTextOutW.Call(hdc, uintptr(int(tx)), uintptr(int(ty)), uintptr(unsafe.Pointer(ptr(text))), uintptr(len([]rune(text))))
	}
	info := fmt.Sprintf("Этикетка %.1f x %.1f мм | рабочая ширина печати %.1f мм | DM %.1f мм", lw, lh, getFloat(ID_MEDIA_W, 26), elems[0].w)
	pTextOutW.Call(hdc, 20, 10, uintptr(unsafe.Pointer(ptr(info))), uintptr(len([]rune(info))))
}
func hitElement(px, py float64) int {
	lw := getFloat(ID_LABEL_W, 15)
	lh := getFloat(ID_LABEL_H, 15)
	scale := minf(440/lw, 500/lh)
	ox, oy := 40.0, 40.0
	mx := (px - ox) / scale
	my := (py - oy) / scale
	for i := 0; i < len(elems); i++ {
		e := elems[i]
		if mx >= e.x && mx <= e.x+e.w && my >= e.y && my <= e.y+e.h {
			return i
		}
	}
	return -1
}
func canvasToMM(px, py float64) (float64, float64) {
	lw := getFloat(ID_LABEL_W, 15)
	lh := getFloat(ID_LABEL_H, 15)
	scale := minf(440/lw, 500/lh)
	return (px - 40) / scale, (py - 40) / scale
}
func syncFromControls() {
	elems[0].x = getFloat(ID_X, 2)
	elems[0].y = getFloat(ID_Y, 2)
	s := getFloat(ID_DM_SIZE, 11)
	elems[0].w = s
	elems[0].h = s
}

func makeFileFilter(parts []string) []uint16 {
	// OPENFILENAME requires embedded NUL separators and a double-NUL terminator.
	// syscall.StringToUTF16 must NOT be used here: it panics on embedded NULs.
	var out []uint16
	for _, part := range parts {
		out = append(out, utf16.Encode([]rune(part))...)
		out = append(out, 0)
	}
	out = append(out, 0)
	return out
}

func openFile(owner syscall.Handle, filterParts []string) string {
	defer func() {
		if r := recover(); r != nil {
			msgbox(fmt.Sprintf("Ошибка окна выбора файла: %v", r), true)
		}
	}()
	buf := make([]uint16, 32768)
	f := makeFileFilter(filterParts)
	title := ptr("Выберите файл")
	ofn := OPENFILENAME{lStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})), hwndOwner: owner, lpstrFilter: &f[0], lpstrFile: &buf[0], nMaxFile: uint32(len(buf)), lpstrTitle: title, Flags: OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST}
	r, _, _ := pOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func getText(id int) string {
	h := controls[id]
	n, _, _ := pGetWindowTextLen.Call(uintptr(h))
	b := make([]uint16, n+1)
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&b[0])), n+1)
	return syscall.UTF16ToString(b)
}
func setText(id int, s string) {
	pSetWindowTextW.Call(uintptr(controls[id]), uintptr(unsafe.Pointer(ptr(s))))
}
func getFloat(id int, d float64) float64 {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(getText(id)), ",", "."), 64)
	if err != nil {
		return d
	}
	return v
}
func setEditFloat(id int, v float64) { setText(id, strconv.FormatFloat(v, 'f', 1, 64)) }
func textEnabled(i int) bool {
	id := ID_TEXT1_CHECK + (i-1)*2
	r, _, _ := pSendMessageW.Call(uintptr(controls[id]), BM_GETCHECK, 0, 0)
	return r == BST_CHECKED
}
func setStatus(s string) { setText(ID_STATUS, s) }
func msgbox(s string, err bool) {
	flag := uintptr(MB_OK | MB_ICONINFORMATION)
	if err {
		flag = MB_OK | MB_ICONERROR
	}
	pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(ptr(s))), uintptr(unsafe.Pointer(ptr("MSK Zebra Label Studio"))), flag)
}

type csvUIEvent struct {
	path  string
	codes [][]byte
	err   error
}

var csvBusy atomic.Bool
var csvEventMu sync.Mutex
var csvEvents = map[uintptr]csvUIEvent{}
var csvEventSeq atomic.Uint64

func postCSVEvent(ev csvUIEvent) {
	id := uintptr(csvEventSeq.Add(1))
	csvEventMu.Lock()
	csvEvents[id] = ev
	csvEventMu.Unlock()
	pPostMessageW.Call(uintptr(hMain), WM_CSV_EVENT, id, 0)
}
func takeCSVEvent(id uintptr) (csvUIEvent, bool) {
	csvEventMu.Lock()
	defer csvEventMu.Unlock()
	v, ok := csvEvents[id]
	if ok {
		delete(csvEvents, id)
	}
	return v, ok
}
func loadCSVAsync(path string) {
	if csvBusy.Swap(true) {
		msgbox("CSV уже загружается.", true)
		return
	}
	setStatus("CSV: " + filepath.Base(path) + " — чтение в фоне...")
	go func() {
		out, err := readCSVCodes(path)
		postCSVEvent(csvUIEvent{path: path, codes: out, err: err})
	}()
}
func readCSVCodes(path string) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, _ := f.Stat()
	capHint := 10000
	if st != nil && st.Size() > 0 { // Typical KM line is around 90-130 bytes.
		est := int(st.Size() / 110)
		if est > capHint {
			capHint = est
		}
		if capHint > 250000 {
			capHint = 250000
		}
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 128*1024), 4*1024*1024)
	out := make([][]byte, 0, capHint)
	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		if first && len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
			line = line[3:]
		}
		first = false
		line = trimCR(line)
		if len(line) == 0 {
			continue
		}
		field := extractCodeField(line)
		if len(field) > 0 {
			out = append(out, append([]byte(nil), field...))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
func splitLinesBytes(b []byte) [][]byte {
	res := [][]byte{}
	start := 0
	for i, c := range b {
		if c == '\n' {
			res = append(res, append([]byte(nil), b[start:i]...))
			start = i + 1
		}
	}
	if start < len(b) {
		res = append(res, append([]byte(nil), b[start:]...))
	}
	return res
}
func trimCR(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		return b[:len(b)-1]
	}
	return b
}
func extractCodeField(line []byte) []byte {
	// Preserve raw bytes whenever the line itself looks like a KM.
	t := line
	if len(t) > 0 && t[0] == '"' {
		r := csv.NewReader(strings.NewReader(string(line)))
		r.FieldsPerRecord = -1
		rec, err := r.Read()
		if err == nil {
			for _, f := range rec {
				bb := []byte(f)
				if looksKM(bb) {
					return bb
				}
			}
			if len(rec) == 1 {
				return []byte(rec[0])
			}
		}
	}
	if looksKM(t) {
		return append([]byte(nil), t...)
	}
	// Try semicolon/tab/comma CSV and select the field resembling a marking code.
	for _, sep := range []byte{';', '\t', ','} {
		r := csv.NewReader(strings.NewReader(string(line)))
		r.Comma = rune(sep)
		r.FieldsPerRecord = -1
		rec, err := r.Read()
		if err == nil && len(rec) > 1 {
			for _, f := range rec {
				bb := []byte(f)
				if looksKM(bb) {
					return bb
				}
			}
		}
	}
	return append([]byte(nil), t...)
}
func looksKM(b []byte) bool { return len(b) >= 20 && b[0] == '0' && b[1] == '1' }
func refreshCodeList() {
	h := controls[ID_CODELIST]
	pSendMessageW.Call(uintptr(h), LB_RESETCONTENT, 0, 0)