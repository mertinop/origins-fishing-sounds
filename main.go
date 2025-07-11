package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	g "xabbo.b7c.io/goearth"
	"xabbo.b7c.io/goearth/shockwave/in"
	"xabbo.b7c.io/goearth/shockwave/profile"
)

var ext = g.NewExt(g.ExtInfo{
	Title:       "origins-fishing-sounds",
	Description: "Play sounds when you stop fishing",
	Author:      "mertinop",
	Version:     "1.0",
})

var (
	userIndex         int
	isFishing         = false
	isPlayingMinigame = false
)

func main() {
	ext.Initialized(onInitialized)
	ext.Connected(onConnected)
	ext.Disconnected(onDisconnected)

	var profileMgr = profile.NewManager((ext))
	ext.Intercept(in.USERS).With(func(e *g.Intercept) {
		cnt := e.Packet.ReadInt()
		log.Printf("[JOIN] User packet has %d user(s)", cnt)
		if cnt == 1 {
			tmpUserIndex := e.Packet.ReadInt()
			e.Packet.ReadInt() // accountId
			tmpUserName := e.Packet.ReadString()
			if tmpUserName == profileMgr.Name {
				log.Printf("Matched User Index: %d Name: %s ProfileName: %s", tmpUserIndex, tmpUserName, profileMgr.Name)
				userIndex = tmpUserIndex
			}
		}
	})

	ext.Intercept(in.STATUS).With(func(e *g.Intercept) {
		totalStatusUpdates := e.Packet.ReadInt()

		for i := 0; i < totalStatusUpdates; i++ {
			userId := e.Packet.ReadInt()
			x := e.Packet.ReadInt()
			y := e.Packet.ReadInt()
			h := e.Packet.ReadString()
			dirHead := e.Packet.ReadInt()
			dirBody := e.Packet.ReadInt()
			actionString := e.Packet.ReadString()
			actions := []string{}
			actionIndex := []string{}
			if actionString != "" {
				actionParts := strings.Split(actionString, "/")
				for _, part := range actionParts {
					if len(part) > 1 {
						actionName := strings.Split(part, " ")[0]
						actions = append(actions, actionName)
						actionIndex = append(actionIndex, actionName)
					}
				}
			}

			if userId == userIndex {
				log.Printf("User ID: %d, X: %d, Y: %d, H: %s, DirHead: %d, DirBody: %d, Actions: %v, ActionIndex: %v",
					userId, x, y, h, dirHead, dirBody, actions, actionIndex)
				wasFishing := isFishing
				if slices.Contains(actions, "fsh") {
					isFishing = true
				} else {
					isFishing = false
				}
				if wasFishing && !isFishing {
					log.Printf("You stopped fishing!")
					go playSound("stopped_fishing.wav")
				}
				log.Printf("isFishing: %v", isFishing)
			}
		}
	})

	ext.InterceptAll(func(e *g.Intercept) {

		// x := e.Packet.ReadInt()
		log.Printf("Intercepted %s message %q\n", e.Dir(), e.Name())

		if e.Name() == "FISHING_STATUS" {
			wasPlayingMinigame := isPlayingMinigame
			isPlayingMinigame = true
			if !wasPlayingMinigame {
				log.Printf("Fishing game started!")
				go playSound("minigame_triggered.wav")
			}
		}
		if e.Name() == "END_FISHING" {
			isPlayingMinigame = false
			log.Printf("Fishing game ended!")
		}

	})

	log.Println("Starting Fishing Sounds... Please re-enter the room")
	ext.Run()
}

func onInitialized(e g.InitArgs) {
	log.Println("Extension initialized")
}

func onConnected(e g.ConnectArgs) {
	log.Printf("Game connected (%s)\n", e.Host)
}

func onDisconnected() {
	log.Println("Game disconnected")
}

const (
	AssetServerBaseURL = "https://github.com/mertinop/origins-fishing-sounds/raw/refs/heads/main/assets/"
)

func playSound(filename string) {
	resp, err := http.Get(AssetServerBaseURL + filename)
	if err != nil {
		log.Println("Error downloading sound file:", err)
		return
	}
	defer resp.Body.Close()

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "sound-*.wav")
	if err != nil {
		log.Println("Error creating temporary file:", err)
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	// Copy the downloaded data to the temporary file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		log.Println("Error writing to temporary file:", err)
		return
	}
	tmpFile.Close()

	// Open the temporary file for playing
	f, err := os.Open(tmpFile.Name())
	log.Printf("Playing sound: %s", tmpFile.Name())
	if err != nil {
		log.Println("Error opening temporary sound file:", err)
		return
	}
	defer f.Close()

	streamer, format, err := wav.Decode(f)
	if err != nil {
		log.Println("Error decoding WAV file:", err)
		return
	}
	defer streamer.Close()

	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		log.Println("Error initializing speaker:", err)
		return
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	<-done
}
