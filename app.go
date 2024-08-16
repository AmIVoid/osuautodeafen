package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"os/exec"
	"time"

	utils "github.com/daftuyda/osuautodeafen/utils"
	"github.com/gorilla/websocket"
	"github.com/micmonay/keybd_event"
)

type ComboTosu struct {
	Current float64 `json:"current"`
	Max     float64 `json:"max"`
}

type BeatmapStatsTosu struct {
	MaxCombo float64 `json:"maxcombo"`
}
type BeatmapTosu struct {
	Stats BeatmapStatsTosu `json:"stats"`
	ID    int              `json:"id"`
	Time  TimeTosu         `json:"time"`
}

type TimeTosu struct {
	Current  float32 `json:"current"`
	Full     float32 `json:"full"`
	Mp3      float32 `json:"mp3"`
	FirstObj float32 `json:"firstObj"`
}
type MenuTosu struct {
	BM    BeatmapTosu `json:"bm"`
	State int         `json:"state"`
}

type GameplayHitsTosu struct {
	Misses      float64 `json:"0"`
	Meh         float64 `json:"50"`
	Okay        float64 `json:"100"`
	Great       float64 `json:"300"`
	SliderBreak float64 `json:"sliderBreaks"`
}
type GameplayTosu struct {
	Name     string           `json:"name"`
	GameMode int              `json:"gamemode"`
	Score    float64          `json:"score"`
	Combo    ComboTosu        `json:"combo"`
	Accuracy float64          `json:"accuracy"`
	Hits     GameplayHitsTosu `json:"hits"`
}

type TosuMemory struct {
	Gameplay GameplayTosu `json:"gameplay"`
	Menu     MenuTosu     `json:"menu"`
	Error    string       `json:"error"`
}

var addr = "localhost:24050"
var alreadyDeafened = false

var recentlyjoined = false
var alreadyDetectedRestart = false
var inbeatmap = false
var misses float64 = 0

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

var unloadedConfig = false

func loadConfig() utils.Settings {
	var cfg utils.Settings
	content, err := os.ReadFile("config.json")
	if err != nil {
		unloadedConfig = true
	}
	json.Unmarshal(content, &cfg)

	return cfg
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
	utils.CheckVersionTosu()
	var config = loadConfig()

	if unloadedConfig {
		fmt.Println("[!] Please configure osu! Auto Deafen in the window that has popped up.")
		utils.CreateWindow(config, true)
		config = loadConfig()
	}
	// check if the argument "--open" is passed, and if so, run the program.
	if len(os.Args) > 1 {
		if os.Args[1] == "--open" {
			var program = os.Args[2]
			fmt.Printf("[#] Opening %s... \n", program)
			// run program async
			go exec.Command(program).Run()
			time.Sleep(4 * time.Second)
		}
	}

	// if start tosu automatically is on, then start process
	cmnd := exec.Command("./deps/tosu.exe")
	if config.General.StartTosuAutomatically {
		fmt.Printf("[#] Starting Tosu... \n")
		cmnd.Start()
		time.Sleep(4 * time.Second)
	}

	deafenKeybind := "alt+d"
	if config.General.DeafenKey != "" {
		deafenKeybind = config.General.DeafenKey
	}

	// Select keys to be pressed
	kb := utils.GenerateKeybonding(deafenKeybind)

	fmt.Printf("[!] Deafen keybind will be %s. Please make sure that your deafen keybind is set to this.\n", deafenKeybind)

	urlParsed := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	ws, _, err := websocket.DefaultDialer.Dial(urlParsed.String(), nil)

	if err != nil {
		fmt.Println("[!!] Error when connecting to TosuMemory. Please make sure that TosuMemory is open and is connected to osu!")
		return
	}
	fmt.Println("[!] Connected to TosuMemory. Make sure that it stays on when playing osu!")

	fmt.Println("[!] Playing as", config.General.Name)

	timesincelastws = time.Now().Unix()

	go func() {
		if utils.State == 0 && !utils.WindowAlreadyOpened {
			utils.CreateWindow(config, false)
			config = loadConfig()
			kb = utils.GenerateKeybonding(deafenKeybind)
		}
		for {
			if time.Now().Unix()-timesincelastws > 1 {
				fmt.Println("[!!] osu! has closed. Now stopping osu! Auto Deafen...")
				if config.General.EnableScreenBlackout {
					utils.RestoreScreens()
				}
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
		var tosuResponse TosuMemory
		jsonerr := json.Unmarshal(message, &tosuResponse)
		if jsonerr != nil {
			fmt.Println("[!!] ", jsonerr)
		} else {

			timesincelastws = time.Now().Unix()

			if tosuResponse.Gameplay.Name == config.General.Name && inbeatmap {

				if tosuResponse.Menu.BM.Time.Current > 1 && (recentlyjoined || alreadyDetectedRestart) {
					recentlyjoined = false
					alreadyDetectedRestart = false
				}

				if tosuResponse.Gameplay.Hits.Misses-misses > 0 || tosuResponse.Gameplay.Hits.SliderBreak+tosuResponse.Gameplay.Hits.Misses != misses {
					//fmt.Println("| Missed, Broke, or lost combo. Incrementing miss count.")
					misses = tosuResponse.Gameplay.Hits.Misses
					if config.Gameplay.CountSliderBreaksAsMiss {
						misses += tosuResponse.Gameplay.Hits.SliderBreak
					}
				}

				if config.Gameplay.UndeafenAfterMisses > 0 {
					if misses >= config.Gameplay.UndeafenAfterMisses && alreadyDeafened {
						fmt.Printf("| Missed too many times (%sx) for undeafen. Now undeafening..\n", fmt.Sprint(config.Gameplay.UndeafenAfterMisses))
						deafenOrUndeafen(kb, false)
					}
				} else if alreadyDeafened {
					// fmt.Println("| Undeafen condition skipped due to UndeafenAfterMisses being set to 0.")
				}

				if tosuResponse.Gameplay.Score == 0 && tosuResponse.Gameplay.Accuracy == 0 && tosuResponse.Gameplay.Combo.Current == 0 && !recentlyjoined && !alreadyDetectedRestart {
					fmt.Println("| Detected that the user has restarted map. Attempting to undeafen..")
					misses = 0
					alreadyDetectedRestart = true
					deafenOrUndeafen(kb, false)
				} else if math.Floor(tosuResponse.Menu.BM.Stats.MaxCombo*config.Gameplay.DeafenPercent) < tosuResponse.Gameplay.Combo.Current && !alreadyDeafened && inbeatmap {
					// Handle RequireFC logic
					if config.Gameplay.RequireFC {
						if misses == 0 {
							fmt.Println("| Reached max combo threshold for map with FC. Now deafening..")
							deafenOrUndeafen(kb, true)
						}
					} else {
						fmt.Println("| Reached max combo threshold for map. Now deafening..")
						deafenOrUndeafen(kb, true)
					}
				}
			}

			if tosuResponse.Menu.State == 2 && utils.State != 2 {
				fmt.Println("[#] Detected Beatmap Join")
				inbeatmap = true
				recentlyjoined = true
				if config.General.EnableScreenBlackout {
					utils.BlackoutScreens()
				}
			} else if utils.State == 2 && tosuResponse.Menu.State != 2 && inbeatmap {
				fmt.Println("[#] Detected Beatmap Exit")
				inbeatmap = false
				misses = 0
				deafenOrUndeafen(kb, false)
				if config.General.EnableScreenBlackout {
					utils.RestoreScreens()
				}
			}
			utils.State = tosuResponse.Menu.State
		}
	}
	shutdown(*cmnd)
}
