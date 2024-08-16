package osuautodeafen

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilectron"
	"github.com/jxeng/shortcut"
	"github.com/ncruces/zenity"
)

type Message struct {
	Type  string `json:"message"`
	Value string `json:"value"`
}

type GeneralSettings struct {
	Name                   string `json:"username"`
	StartTosuAutomatically bool   `json:"starttosu"`
	DeafenKey              string `json:"deafenkey"`
	EnableScreenBlackout   bool   `json:"blackout"`
}

type GameplaySettings struct {
	DeafenPercent           float64 `json:"deafenpercent"`
	UndeafenAfterMisses     float64 `json:"undeafenmiss"`
	CountSliderBreaksAsMiss bool    `json:"countsliderbreakmiss"`
	RequireFC               bool    `json:"requirefc"`
}
type Settings struct {
	Gameplay GameplaySettings `json:"gameplay"`
	General  GeneralSettings  `json:"general"`
}
type SettingAsMessage struct {
	Type  string   `json:"type"`
	Value Settings `json:"value"`
}

var State int = 0
var WindowAlreadyOpened = false

var resources = []string{
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/resources/app/app.js",
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/resources/app/index.html",
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/resources/app/style.css",
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/resources/app/slider.css",
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/assets/logo-not-transparent.png",
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/resources/app/version.txt",
	"https://raw.githubusercontent.com/daftuyda/osuautodeafen/master/resources/app/osu.ico",
}

func DownloadResources() {
	// download the resources
	fmt.Println("[#] Downloading resources..")
	// check if a resources folder exists
	if _, err := os.Stat("./resources/"); os.IsNotExist(err) {
		os.Mkdir("./resources", os.ModeAppend)
		os.Mkdir("./resources/app", os.ModeAppend)
	}
	for _, resource := range resources {
		resp, err := http.Get(resource)
		if err != nil {
			fmt.Println("[!!] Error occurred when downloading resources.")
			return
		}

		defer resp.Body.Close()
		bodyEncoded, _ := io.ReadAll(resp.Body)
		// create the file in the resources/app
		var fileName = resource[strings.LastIndex(resource, "/")+1:]
		out, err := os.Create("./resources/app/" + fileName)
		if err != nil {
			fmt.Println("[!!] Error occurred when creating file.")
			return
		}
		defer out.Close()
		// write the body to the file
		out.Write(bodyEncoded)
		fmt.Println("[#] Downloaded " + fileName)
	}
	fmt.Println("[#] Finished downloading resources.")
}

func checkVersionAndDownloadResources() {
	// Check if the version.txt file exists in resources/app
	_, err := os.Stat("./resources/app/version.txt")
	if os.IsNotExist(err) {
		fmt.Println("[!!] Error occurred when checking version.")
		return
	}
	// Read the version file
	versionFile, err := os.Open("./resources/app/version.txt")
	if err != nil {
		fmt.Println("[!!] Error occurred when opening version file.")
		return
	}
	defer versionFile.Close()

	version, err := io.ReadAll(versionFile)
	if err != nil {
		fmt.Println("[!!] Error occurred when reading version file.")
		return
	}

	// Check if the version is the same as the one online
	resp, err := http.Get("https://raw.githubusercontent.com/daftuyda/osuautodeafen/future/resources/app/version.txt")
	if err != nil {
		fmt.Println("[!!] Error occurred when checking version online.")
		return
	}
	defer resp.Body.Close()
	bodyEncoded, _ := io.ReadAll(resp.Body)

	if string(version) != string(bodyEncoded) {
		fmt.Println("[#] New version available, downloading..")
		DownloadResources()
	}
}

func handleMessages(w *astilectron.Window, settings Settings, isFirstLoad bool) {
	// Prepare the initial message
	settingTypeName := "load"
	if isFirstLoad {
		settingTypeName += "-FIRSTLOAD"
	}
	message := SettingAsMessage{Type: settingTypeName, Value: settings}
	loadSettingsOut, _ := json.Marshal(message)

	// Send the initial settings message
	w.SendMessage(string(loadSettingsOut), func(m *astilectron.EventMessage) {
		var s string
		m.Unmarshal(&s)
		fmt.Printf("[#] %s\n", s)
	})

	// Handle incoming messages
	w.OnMessage(func(m *astilectron.EventMessage) (v interface{}) {
		var s string
		m.Unmarshal(&s)
		var message SettingAsMessage
		json.Unmarshal([]byte(s), &message)

		switch message.Type {
		case "generate-shortcut":
			// Ask the user for the location of the "osu!.exe" file
			username := os.Getenv("USERNAME")
			fileDialog, diagErr := zenity.SelectFile(
				zenity.Title("Select osu!.exe"),
				zenity.FileFilters{
					zenity.FileFilter{
						Name: "osu!.exe",
					},
				},
				zenity.Filename("C:\\"+username+"\\AppData\\Local\\osu!\\osu!.exe"),
			)
			if diagErr != nil {
				fmt.Println("[!!] Error occurred when opening file dialog.")
				return "ERROR"
			}

			// Get the path of the file
			path := fileDialog

			// Get the local path of this .exe file
			ex, err := os.Executable()
			if err != nil {
				panic(err)
			}
			exPath := filepath.Dir(ex) + "\\" + filepath.Base(ex)

			// Create the shortcut
			generatedShortcut := shortcut.Shortcut{
				ShortcutPath:     "C:\\Users\\" + username + "\\AppData\\Roaming\\Microsoft\\Windows\\Start Menu\\Programs\\osu! Auto Deafen.lnk",
				Target:           exPath,
				IconLocation:     filepath.Dir(ex) + "\\resources\\app\\osu.ico",
				Arguments:        "--open \"" + path + "\"",
				WorkingDirectory: filepath.Dir(ex),
			}
			shortcut.Create(generatedShortcut)
			return "SUCCESS"

		case "saveclose":
			// Close the window
			fmt.Println("[#] Close request received, closing the window.")
			w.Close()
		}

		// Save settings to config.json
		remarshal, _ := json.Marshal(message.Value)
		if message.Value.General.DeafenKey == "" {
			message.Value.General.DeafenKey = "alt+d"
		}

		out, _ := os.Create("config.json")
		out.Write([]byte(remarshal))
		out.Close()

		return "SUCCESS"
	})
}

func CreateWindow(settings Settings, isFirstLoad bool) {
	if WindowAlreadyOpened {
		fmt.Println("[#] Window already opened, skipping creation.")
		return
	}

	// Check if resources are available
	_, err := os.Stat("./resources/app/index.html")
	if os.IsNotExist(err) {
		fmt.Println("[#] Preparing to download resources..")
		DownloadResources()
	}

	// Version check and download resources if necessary
	checkVersionAndDownloadResources()

	// Create new Astilectron instance
	a, err := astilectron.New(nil, astilectron.Options{
		AppName:            "osuautodeafen",
		AppIconDefaultPath: "./resources/icon.png",
		VersionAstilectron: "0.33.0",
		VersionElectron:    "4.0.1",
	})
	if err != nil {
		fmt.Println("[!!] Error occurred when creating Astilectron instance:", err)
		return
	}
	defer a.Close()

	// Start Astilectron
	if err = a.Start(); err != nil {
		fmt.Println("[!!] Error occurred when starting Astilectron:", err)
		return
	}

	// Create window
	w, err := a.NewWindow("./resources/app/index.html", &astilectron.WindowOptions{
		Height:      astikit.IntPtr(245),
		Width:       astikit.IntPtr(195),
		AlwaysOnTop: astikit.BoolPtr(true),
		Transparent: astikit.BoolPtr(true),
		Frame:       astikit.BoolPtr(false),
		Resizable:   astikit.BoolPtr(false),
		X:           astikit.IntPtr(15),
		Y:           astikit.IntPtr(100),
	})
	if err != nil {
		fmt.Println("[!!] Error occurred when creating window:", err)
		return
	}
	WindowAlreadyOpened = true

	// Debug print: Window created
	fmt.Println("[#] Window created successfully.")

	// Create window
	if err = w.Create(); err != nil {
		fmt.Println("[!!] Error occurred when creating window:", err)
		return
	}

	// Handle window close event using the "close" event name
	w.On("close", func(e astilectron.Event) (deleteListener bool) {
		fmt.Println("[#] Window close event triggered.")
		WindowAlreadyOpened = false

		// Debug before quitting
		fmt.Println("[#] Calling a.Quit() to terminate the Electron process.")

		// Quit Astilectron and clean up resources
		if err := a.Quit(); err != nil {
			fmt.Println("[!!] Error occurred when quitting Astilectron:", err)
		} else {
			fmt.Println("[#] Electron process terminated successfully.")
		}

		return true
	})

	// Handle messages between Go and Electron
	handleMessages(w, settings, isFirstLoad)

	// Blocking pattern
	a.Wait()
}
