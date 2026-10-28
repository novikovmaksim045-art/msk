//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

//go:embed ZebraLabelStudio.exe PDFDecoder.exe PrinterWorker.exe runtime.zip
var payload embed.FS

const appVersion = "1.2"

var (
	ole32             = syscall.NewLazyDLL("ole32.dll")
	shell32           = syscall.NewLazyDLL("shell32.dll")
	user32            = syscall.NewLazyDLL("user32.dll")
	pCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	pCoUninitialize   = ole32.NewProc("CoUninitialize")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
	pMessageBoxW      = user32.NewProc("MessageBoxW")
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}
type IUnknown struct{ Vtbl *[64]uintptr }

var (
	CLSID_ShellLink  = GUID{0x00021401, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	IID_IShellLinkW  = GUID{0x000214F9, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	IID_IPersistFile = GUID{0x0000010b, 0, 0, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

func w(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func msg(text string, err bool) {
	flags := uintptr(0x40)
	if err {
		flags = 0x10
	}
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(w(text))), uintptr(unsafe.Pointer(w("MSK Zebra Label Studio Setup"))), flags)
}

func main() {
	b, err := payload.ReadFile("ZebraLabelStudio.exe")
	if err != nil {
		msg(err.Error(), true)
		return
	}
	helper, err := payload.ReadFile("PDFDecoder.exe")
	if err != nil {
		msg(err.Error(), true)
		return
	}
	printerWorker, err := payload.ReadFile("PrinterWorker.exe")
	if err != nil {
		msg(err.Error(), true)
		return
	}
	rz, err := payload.ReadFile("runtime.zip")
	if err != nil {
		msg(err.Error(), true)
		return
	}

	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	appDir := filepath.Join(base, "MSK", "Zebra Label Studio")

	// Upgrade path: close any running old build first. No console window is shown.
	closeOldInstances()
	time.Sleep(350 * time.Millisecond)

	// Remove obsolete program/runtime files so an upgrade cannot leave mixed DLLs
	// from previous versions. User settings are intentionally preserved if a
	// future version creates settings.json/templates.json.
	if err = cleanInstallDir(appDir); err != nil {
		msg("Не удалось обновить предыдущую версию. Закройте MSK Zebra Label Studio и запустите установщик ещё раз.\n\n"+err.Error(), true)
		return
	}
	if err = os.MkdirAll(appDir, 0755); err != nil {
		msg(err.Error(), true)
		return
	}

	exePath := filepath.Join(appDir, "ZebraLabelStudio.exe")
	if err = os.WriteFile(exePath, b, 0755); err != nil {
		msg(err.Error(), true)
		return
	}
	if err = os.WriteFile(filepath.Join(appDir, "PDFDecoder.exe"), helper, 0755); err != nil {
		msg(err.Error(), true)
		return
	}
	if err = os.WriteFile(filepath.Join(appDir, "PrinterWorker.exe"), printerWorker, 0755); err != nil {
		msg(err.Error(), true)
		return
	}
	if err = extractZipBytes(rz, appDir); err != nil {
		msg("Не удалось установить модуль PDF/DataMatrix:\n\n"+err.Error(), true)
		return
	}
	_ = os.WriteFile(filepath.Join(appDir, "version.txt"), []byte(appVersion+"\r\n"), 0644)

	removeLegacyShortcuts()
	_ = createShortcuts(exePath, appDir)
	registerUninstall(appDir)

	// Launch the new build and immediately exit the installer. Do NOT display a
	// final message box here: it used to steal focus and could leave the app
	// running behind other windows.
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(w("open"))), uintptr(unsafe.Pointer(w(exePath))), 0, uintptr(unsafe.Pointer(w(appDir))), 5)
}

func closeOldInstances() {
	for _, image := range []string{"ZebraLabelStudio.exe", "PDFDecoder.exe", "PrinterWorker.exe"} {
		cmd := exec.Command("taskkill.exe", "/IM", image, "/F", "/T")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		_ = cmd.Run()
	}
}

func cleanInstallDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	keep := map[string]bool{"settings.json": true, "templates.json": true, "journals": true}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if keep[strings.ToLower(e.Name())] {
			continue
		}
		path := filepath.Join(dir, e.Name())
		var last error
		for i := 0; i < 5; i++ {
			last = os.RemoveAll(path)
			if last == nil {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if last != nil {
			return fmt.Errorf("не удалось удалить %s: %w", path, last)
		}
	}
	return nil
}

func removeLegacyShortcuts() {
	paths := []string{
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "MSK Zebra Label Studio.lnk"),
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "Zebra Label Studio.lnk"),
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "MSK Zebra Label Studio.lnk"),
		filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Zebra Label Studio.lnk"),
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

func registerUninstall(appDir string) {
	// Per-user Add/Remove Programs registration, no elevation required.
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\MSKZebraLabelStudio`
	exePath := filepath.Join(appDir, "ZebraLabelStudio.exe")
	values := [][]string{
		{"ADD", key, "/v", "DisplayName", "/t", "REG_SZ", "/d", "MSK Zebra Label Studio", "/f"},
		{"ADD", key, "/v", "DisplayVersion", "/t", "REG_SZ", "/d", appVersion, "/f"},
		{"ADD", key, "/v", "InstallLocation", "/t", "REG_SZ", "/d", appDir, "/f"},
		{"ADD", key, "/v", "DisplayIcon", "/t", "REG_SZ", "/d", exePath, "/f"},
	}
	for _, args := range values {
		cmd := exec.Command("reg.exe", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		_ = cmd.Run()
	}
}

func extractZipBytes(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		clean := filepath.Clean(f.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("некорректный путь в архиве: %s", f.Name)
		}
		target := filepath.Join(dest, clean)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, cpErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if cpErr != nil {
			return cpErr
		}
	}
	return nil
}

func createShortcuts(exe, work string) error {
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "MSK Zebra Label Studio.lnk")
	start := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "MSK Zebra Label Studio.lnk")
	if err := createLink(exe, work, desktop); err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(start), 0755)
	return createLink(exe, work, start)
}

func createLink(target, work, link string) error {
	hr, _, _ := pCoInitializeEx.Call(0, 2)
	_ = hr
	defer pCoUninitialize.Call()
	var sl *IUnknown
	r, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&CLSID_ShellLink)), 0, 1, uintptr(unsafe.Pointer(&IID_IShellLinkW)), uintptr(unsafe.Pointer(&sl)))
	if int32(r) < 0 || sl == nil {
		return fmt.Errorf("не удалось создать ярлык: 0x%X", r)
	}
	defer syscall.SyscallN(sl.Vtbl[2], uintptr(unsafe.Pointer(sl)))
	syscall.SyscallN(sl.Vtbl[20], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(w(target))))
	syscall.SyscallN(sl.Vtbl[9], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(w(work))))
	syscall.SyscallN(sl.Vtbl[7], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(w("Печать кодов маркировки GS1 DataMatrix на Zebra"))))
	syscall.SyscallN(sl.Vtbl[17], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(w(target))), 0)
	var pf *IUnknown
	qr, _, _ := syscall.SyscallN(sl.Vtbl[0], uintptr(unsafe.Pointer(sl)), uintptr(unsafe.Pointer(&IID_IPersistFile)), uintptr(unsafe.Pointer(&pf)))
	if int32(qr) < 0 || pf == nil {
		return fmt.Errorf("IPersistFile: 0x%X", qr)
	}
	defer syscall.SyscallN(pf.Vtbl[2], uintptr(unsafe.Pointer(pf)))
	sr, _, _ := syscall.SyscallN(pf.Vtbl[6], uintptr(unsafe.Pointer(pf)), uintptr(unsafe.Pointer(w(link))), 1)
	if int32(sr) < 0 {
		return fmt.Errorf("сохранение ярлыка: 0x%X", sr)
	}
	return nil
}