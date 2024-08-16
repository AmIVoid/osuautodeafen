package osuautodeafen

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	procFillRect            = user32.NewProc("FillRect")
	procCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procIsWindow            = user32.NewProc("IsWindow")
)

var (
	blackoutWindows []win.HWND
	classRegistered bool
	className       *uint16
	hInstance       win.HINSTANCE
)

func init() {
	hInstance = win.GetModuleHandle(nil)
	className = mustUTF16PtrFromString("osuAutoDeafenWindowClass")
}

// BlackoutScreens creates blackout windows on all monitors, focusing on the one where osu! is running
func BlackoutScreens() {
	if !classRegistered {
		registerWindowClass() // Register the window class if not already done
	}

	osuWindowName := mustUTF16PtrFromString("osu!")
	osuWindow := win.FindWindow(nil, osuWindowName)
	if osuWindow == 0 {
		return
	}

	hMonitor := win.MonitorFromWindow(osuWindow, win.MONITOR_DEFAULTTONEAREST)

	var osuRect win.RECT
	if ret, _, _ := procGetWindowRect.Call(uintptr(osuWindow), uintptr(unsafe.Pointer(&osuRect))); ret == 0 {
		panic("Failed to get osu! window")
	}

	// Create a blackout window on the same monitor or position it below osu!
	go func() {
		createBlackoutWindow(hMonitor, osuWindow)
	}()

	// Create blackout windows on other monitors
	callback := syscall.NewCallback(func(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
		if hMonitor != win.MonitorFromWindow(osuWindow, win.MONITOR_DEFAULTTONEAREST) {
			go func() {
				createFullScreenBlackout(hMonitor)
			}()
		}
		return 1 // Continue enumeration
	})

	ret, _, _ := procEnumDisplayMonitors.Call(0, 0, callback, 0)
	if ret == 0 {
		// Failed to enumerate display monitors
	}
}

// RestoreScreens restores the screens by closing all blackout windows
func RestoreScreens() {
	for _, hwnd := range blackoutWindows {
		if hwnd != 0 && isWindowValid(hwnd) {
			win.SendMessage(hwnd, win.WM_CLOSE, 0, 0)
			time.Sleep(100 * time.Millisecond)
		}
	}

	blackoutWindows = nil

	// Redraw the desktop to refresh the display
	win.RedrawWindow(0, nil, 0, win.RDW_INVALIDATE|win.RDW_UPDATENOW|win.RDW_ALLCHILDREN)

	// Unregister the window class if it was registered
	if classRegistered {
		if win.UnregisterClass(className) {
			classRegistered = false
		}
	}
}

// createBlackoutWindow creates a blackout window on a specific monitor
func createBlackoutWindow(hMonitor win.HMONITOR, osuWindow win.HWND) {
	var monitorInfo win.MONITORINFO
	monitorInfo.CbSize = uint32(unsafe.Sizeof(monitorInfo))
	if !win.GetMonitorInfo(hMonitor, &monitorInfo) {
		return // Failed to get monitor info
	}

	monitorRect := monitorInfo.RcMonitor

	// Create the blackout window
	hwnd := win.CreateWindowEx(
		win.WS_EX_TOPMOST|win.WS_EX_NOACTIVATE,
		className,
		mustUTF16PtrFromString("Blackout"),
		win.WS_POPUP|win.WS_VISIBLE,
		monitorRect.Left, monitorRect.Top, monitorRect.Right-monitorRect.Left, monitorRect.Bottom-monitorRect.Top,
		0, 0, hInstance, nil,
	)

	if hwnd == 0 {
		return // Failed to create window
	}

	// Position the window below osu! in the Z-order
	win.SetWindowPos(hwnd, osuWindow, 0, 0, 0, 0, win.SWP_NOSIZE|win.SWP_NOMOVE|win.SWP_NOACTIVATE)
	win.ShowWindow(hwnd, win.SW_SHOWNA)
	win.UpdateWindow(hwnd)

	// Add the window handle to the slice of blackout windows
	blackoutWindows = append(blackoutWindows, hwnd)

	go runMessageLoop(hwnd) // Start a message loop to keep the window responsive
}

// createFullScreenBlackout creates a fullscreen blackout window on a specific monitor
func createFullScreenBlackout(hMonitor win.HMONITOR) {
	var monitorInfo win.MONITORINFO
	monitorInfo.CbSize = uint32(unsafe.Sizeof(monitorInfo))
	if !win.GetMonitorInfo(hMonitor, &monitorInfo) {
		panic("Failed to get monitor info")
	}

	monitorRect := monitorInfo.RcMonitor

	// Create the fullscreen blackout window
	hwnd := win.CreateWindowEx(
		win.WS_EX_TOPMOST|win.WS_EX_NOACTIVATE,
		className,
		mustUTF16PtrFromString("Blackout"),
		win.WS_POPUP|win.WS_VISIBLE|win.WS_DISABLED,
		monitorRect.Left, monitorRect.Top, monitorRect.Right-monitorRect.Left, monitorRect.Bottom-monitorRect.Top,
		0, 0, hInstance, nil,
	)
	if hwnd == 0 {
		return // Failed to create window
	}

	win.ShowWindow(hwnd, win.SW_SHOWNA)
	win.UpdateWindow(hwnd)

	blackoutWindows = append(blackoutWindows, hwnd)

	go runMessageLoop(hwnd) // Start a message loop to keep the window responsive
}

func runMessageLoop(hwnd win.HWND) {
	var msg win.MSG
	for {
		ret := win.GetMessage(&msg, hwnd, 0, 0)
		if ret == 0 {
			break // Exit the loop if the message loop ends
		} else if ret == -1 {
			break // Exit the loop if there's an error
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func registerWindowClass() {
	blackBrush := CreateSolidBrush(0x000000) // Black color brush

	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		HbrBackground: blackBrush, // Set the background brush to black
		LpszClassName: className,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
	}

	if atom := win.RegisterClassEx(&wc); atom == 0 {
		errorCode := win.GetLastError()
		if errorCode != 1410 { // ERROR_CLASS_ALREADY_EXISTS
			panic("Failed to register window class")
		}
	} else {
		classRegistered = true
	}
}

// wndProc is the window procedure that handles messages sent to the blackout windows
func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		hdc := win.BeginPaint(hwnd, &ps)
		defer win.EndPaint(hwnd, &ps)

		rect := win.RECT{}
		win.GetClientRect(hwnd, &rect)
		blackBrush := CreateSolidBrush(0x000000)
		FillRect(hdc, &rect, blackBrush)

		return 0

	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func CreateSolidBrush(color win.COLORREF) win.HBRUSH {
	ret, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return win.HBRUSH(ret)
}

func FillRect(hdc win.HDC, rect *win.RECT, brush win.HBRUSH) int32 {
	ret, _, _ := procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(rect)), uintptr(brush))
	return int32(ret)
}

func mustUTF16PtrFromString(s string) *uint16 {
	ptr, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return ptr
}

// isWindowValid checks if a window handle is still valid
func isWindowValid(hwnd win.HWND) bool {
	ret, _, _ := procIsWindow.Call(uintptr(hwnd))
	return ret != 0
}
