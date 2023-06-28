package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	utils "github.com/Nat3z/osudeafen/utils"
	"github.com/gorilla/websocket"
	"github.com/micmonay/keybd_event"
	"gopkg.in/ini.v1"
)

type ComboGosu struct {
	Current float64 `json:"current"`
	Max     float64 `json:"max"`
}

type BeatmapStatsGosu struct {
	MaxCombo float64 `json:"maxcombo"`
}
type BeatmapGosu struct {
	Stats BeatmapStatsGosu `json:"stats"`
	ID    int              `json:"id"`
	Time  TimeGosu         `json:"time"`
}

type TimeGosu struct {
	Current  float32 `json:"current"`
	Full     float32 `json:"full"`
	FirstObj float32 `json:"firstObj"`
}
type MenuGosu struct {
	BM    BeatmapGosu `json:"bm"`
	State int         `json:"state"`
}

type GameplayHitsGosu struct {
	Misses      float64 `json:"0"`
	Meh         float64 `json:"50"`
	Okay        float64 `json:"100"`
	Great       float64 `json:"300"`
	SliderBreak float64 `json:"sliderBreaks"`
}
type GameplayGosu struct {
	Name     string           `json:"name"`
	GameMode int              `json:"gamemode"`
	Score    float64          `json:"score"`
	Combo    ComboGosu        `json:"combo"`
	Accuracy float64          `json:"accuracy"`
	Hits     GameplayHitsGosu `json:"hits"`
}

type GoSuMemory struct {
	Gameplay GameplayGosu `json:"gameplay"`
	Menu     MenuGosu     `json:"menu"`
	Error    string       `json:"error"`
}

type GeneralSettings struct {
	Name                         string `ini:"username"`
	StartGosuMemoryAutomatically bool   `ini:"startgosumemory"`
	Keybind                      string `ini:"keybind"`
}

type GameplaySettings struct {
	DeafenPercent       float64 `ini:"deafenpercent"`
	UndeafenAfterMisses float64 `ini:"undeafenmiss"`
	ToggleScoreboard    bool    `ini:"scoretoggle"`
	ToggleUI            bool    `ini:"uitoggle"`
	ClipFC              bool    `ini:"clipfc"`
}

type Settings struct {
	Gameplay GameplaySettings `ini:"gameplay"`
	General  GeneralSettings  `ini:"general"`
}

var addr = "localhost:24050"
var alreadyDeafened = false
var alreadyHidden = false

var state int = 0
var recentlyjoined = false
var alreadyDetectedRestart = false
var inbeatmap = false
var misses float64 = 0

var KbMap = map[string][]int{
	// function keys
	"F1":  {keybd_event.VK_F1},
	"F2":  {keybd_event.VK_F2},
	"F3":  {keybd_event.VK_F3},
	"F4":  {keybd_event.VK_F4},
	"F5":  {keybd_event.VK_F5},
	"F6":  {keybd_event.VK_F6},
	"F7":  {keybd_event.VK_F7},
	"F8":  {keybd_event.VK_F8},
	"F9":  {keybd_event.VK_F9},
	"F10": {keybd_event.VK_F10},
	"F11": {keybd_event.VK_F11},
	"F12": {keybd_event.VK_F12},

	// numbers
	"1": {keybd_event.VK_1},
	"2": {keybd_event.VK_2},
	"3": {keybd_event.VK_3},
	"4": {keybd_event.VK_4},
	"5": {keybd_event.VK_5},
	"6": {keybd_event.VK_6},
	"7": {keybd_event.VK_7},
	"8": {keybd_event.VK_8},
	"9": {keybd_event.VK_9},
	"0": {keybd_event.VK_0},

	// uppercase
	"Q": {keybd_event.VK_Q},
	"W": {keybd_event.VK_W},
	"E": {keybd_event.VK_E},
	"R": {keybd_event.VK_R},
	"T": {keybd_event.VK_T},
	"Y": {keybd_event.VK_Y},
	"U": {keybd_event.VK_U},
	"I": {keybd_event.VK_I},
	"O": {keybd_event.VK_O},
	"P": {keybd_event.VK_P},
	"A": {keybd_event.VK_A},
	"S": {keybd_event.VK_S},
	"D": {keybd_event.VK_D},
	"F": {keybd_event.VK_F},
	"G": {keybd_event.VK_G},
	"H": {keybd_event.VK_H},
	"J": {keybd_event.VK_J},
	"K": {keybd_event.VK_K},
	"L": {keybd_event.VK_L},
	"Z": {keybd_event.VK_Z},
	"X": {keybd_event.VK_X},
	"C": {keybd_event.VK_C},
	"V": {keybd_event.VK_V},
	"B": {keybd_event.VK_B},
	"N": {keybd_event.VK_N},
	"M": {keybd_event.VK_M},

	// lowercase
	"q": {keybd_event.VK_Q},
	"w": {keybd_event.VK_W},
	"e": {keybd_event.VK_E},
	"r": {keybd_event.VK_R},
	"t": {keybd_event.VK_T},
	"y": {keybd_event.VK_Y},
	"u": {keybd_event.VK_U},
	"i": {keybd_event.VK_I},
	"o": {keybd_event.VK_O},
	"p": {keybd_event.VK_P},
	"a": {keybd_event.VK_A},
	"s": {keybd_event.VK_S},
	"d": {keybd_event.VK_D},
	"f": {keybd_event.VK_F},
	"g": {keybd_event.VK_G},
	"h": {keybd_event.VK_H},
	"j": {keybd_event.VK_J},
	"k": {keybd_event.VK_K},
	"l": {keybd_event.VK_L},
	"z": {keybd_event.VK_Z},
	"x": {keybd_event.VK_X},
	"c": {keybd_event.VK_C},
	"v": {keybd_event.VK_V},
	"b": {keybd_event.VK_B},
	"n": {keybd_event.VK_N},
	"m": {keybd_event.VK_M},
}

// true for deafen
// false for undeafen
func deafenOrUndeafen(kb keybd_event.KeyBonding, expect bool) {

	if alreadyDeafened {
		// if expecting a deafen, dont do anything.
		if expect {
			return
		}
		fmt.Println("| [KP] UNDEAFEN")
		kb.Launching()
	} else {
		// if expecting an undeafen, dont do anything.
		if !expect {
			return
		}
		fmt.Println("| [KP] DEAFEN")
		kb.Launching()
	}

	alreadyDeafened = !alreadyDeafened
}

func toggleUI(uiToggle keybd_event.KeyBonding, expect bool) {

	uiToggle.SetKeys(keybd_event.VK_TAB)
	uiToggle.HasSHIFT(true)

	if loadConfig().Gameplay.ToggleUI == false {

	} else {
		if alreadyHidden {
			if expect {
				return
			}
			fmt.Println("| [KP] Toggle UI On")
			uiToggle.Press()
			time.Sleep(10 * time.Millisecond)
			uiToggle.Release()
		} else {
			if !expect {
				return
			}
			fmt.Println("| [KP] Toggle UI Off")
			uiToggle.Press()
			time.Sleep(10 * time.Millisecond)
			uiToggle.Release()
		}
		alreadyHidden = !alreadyHidden
	}

}

func toggleScore(scoreToggle keybd_event.KeyBonding) {

	scoreToggle.SetKeys(keybd_event.VK_TAB)

	if loadConfig().Gameplay.ToggleScoreboard == false {
	} else {
		fmt.Println("| [KP] Toggling scoreboard")
		scoreToggle.Press()
		time.Sleep(10 * time.Millisecond)
		scoreToggle.Release()
	}

}

func clipFC(clipKeybind keybd_event.KeyBonding) {

	clipKeybind.SetKeys(keybd_event.VK_HOME)
	clipKeybind.HasALT(true)

	if loadConfig().Gameplay.ClipFC == false {
	} else {
		fmt.Println("| [KP] Clipping play")
		clipKeybind.Press()
		time.Sleep(10 * time.Millisecond)
		clipKeybind.Release()
	}

}

func loadConfig() Settings {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		fmt.Println("[!!] No config.ini found! Creating a config.ini...")
		out, _ := os.Create("config.ini")
		resp, err := http.Get("https://raw.githubusercontent.com/amivoid/osuautodeafen/master/config.ini.temp")
		if err != nil {
			fmt.Println("[!!] Unable to get template for osuautodeafen. Please connect to the internet and try again later.")
			os.Exit(1)
		}
		// tempout, _ := os.ReadFile("config.ini.temp")
		// temp := string(tempout)
		temp, _ := io.ReadAll(resp.Body)
		out.Write(([]byte)(temp))
		out.Close()
		fmt.Println("[#] Config.ini has been created! Please setup the config file and launch osuautodeafen.")
		time.Sleep(5 * time.Second)
		os.Exit(0)
		return Settings{}
	}
	var settings = new(Settings)
	cfg.MapTo(&settings)

	return *settings
}

func shutdown(cmnd exec.Cmd) {
	if err := cmnd.Process.Kill(); err != nil {
		log.Fatal("failed to kill process: ", err)
	}
	os.Exit(0)
}

var timesincelastws int64 = 0

func main() {
	fmt.Printf("[#] Checking for Updates...\n")
	utils.CheckVersion()
	utils.CheckVersionGosu()
	var config = loadConfig()

	// if start gosumemory automatically is on, then start process
	cmnd := exec.Command("./deps/gosumemory.exe")
	if config.General.StartGosuMemoryAutomatically {
		fmt.Printf("[#] Starting GosuMemory... \n")
		cmnd.Start()
		time.Sleep(2 * time.Second)
	}

	deafenKeybind := config.General.Keybind
	kb, err := keybd_event.NewKeyBonding()
	uiToggle, err := keybd_event.NewKeyBonding()
	scoreToggle, err := keybd_event.NewKeyBonding()
	clipKeybind, err := keybd_event.NewKeyBonding()

	if err != nil {
		panic(err)
	}

	// Select keys to be pressed
	vks := KbMap[string(deafenKeybind)]
	kb.SetKeys(vks...)
	kb.HasALT(true)

	fmt.Printf("[!] Deafen keybind will be ALT+%s. Please make sure that your deafen keybind is set to this.\n", deafenKeybind)

	urlParsed := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	ws, _, err := websocket.DefaultDialer.Dial(urlParsed.String(), nil)

	if err != nil {
		fmt.Println("[!!] Error when connecting to GosuMemory. Please make sure that GosuMemory is open and is connected to osu!")
		shutdown(*cmnd)
		return
	}
	fmt.Println("[!] Connected to GosuMemory. Make sure that it stays on when playing osu!")
	fmt.Println("[!] Playing as", config.General.Name)

	timesincelastws = time.Now().Unix()

	go func() {
		for {
			if time.Now().Unix()-timesincelastws > 1 {
				fmt.Println("[!!] osu! has closed. Now stopping osu! Auto Deafen...")
				shutdown(*cmnd)
				break
			}
		}
	}()
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			fmt.Println("[!!] Error reading: ", err)
			break
		}
		var gosuResponse GoSuMemory
		jsonerr := json.Unmarshal(message, &gosuResponse)
		if jsonerr != nil {
			fmt.Println("[!!] ", jsonerr)
		} else {

			timesincelastws = time.Now().Unix()

			if gosuResponse.Gameplay.Name == config.General.Name && inbeatmap {

				if gosuResponse.Menu.BM.Time.Current > 1 && (recentlyjoined || alreadyDetectedRestart) {
					recentlyjoined = false
					alreadyDetectedRestart = false
				}

				if gosuResponse.Gameplay.Hits.Misses-misses != 0 {
					fmt.Println("| Missed, Broke, or lost combo. Incrementing miss count.")
					misses = gosuResponse.Gameplay.Hits.Misses
				}

				if misses >= config.Gameplay.UndeafenAfterMisses && alreadyDeafened {
					fmt.Printf("| Missed too many times (%sx) for undeafen. Now undeafening..\n", fmt.Sprint(config.Gameplay.UndeafenAfterMisses))
					deafenOrUndeafen(kb, false)
					toggleUI(uiToggle, false)
				}

				if gosuResponse.Gameplay.Score == 0 && gosuResponse.Gameplay.Accuracy == 0 && gosuResponse.Gameplay.Combo.Current == 0 && !recentlyjoined && !alreadyDetectedRestart {
					fmt.Println("| Detected that the user has restarted map. Attempting to undeafen..")
					misses = 0
					alreadyDetectedRestart = true
					deafenOrUndeafen(kb, false)
					toggleUI(uiToggle, false)
				} else if math.Floor(gosuResponse.Menu.BM.Stats.MaxCombo*config.Gameplay.DeafenPercent) < gosuResponse.Gameplay.Combo.Current && !alreadyDeafened && inbeatmap && misses == 0 {
					fmt.Println("| Reached max combo treshold for map. Now deafening..")
					deafenOrUndeafen(kb, true)
					toggleUI(uiToggle, true)
					toggleScore(scoreToggle)
				}
			}

			if gosuResponse.Menu.State == 2 && state != 2 {
				fmt.Println("[#] Detected Beatmap Join")
				inbeatmap = true
				recentlyjoined = true
			} else if state == 2 && gosuResponse.Menu.State != 2 && inbeatmap {

				if misses == 0 && gosuResponse.Menu.State == 7 {
					clipFC(clipKeybind)
				}

				fmt.Println("[#] Detected Beatmap Exit")
				inbeatmap = false
				misses = 0
				deafenOrUndeafen(kb, false)
			}
			state = gosuResponse.Menu.State
		}
	}
	shutdown(*cmnd)
}
