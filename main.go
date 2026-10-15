package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"uuid"

	"github.com/joho/godotenv"
)

const (
	PLAYING_TYPE   = 0
	LISTENING_TYPE = 2
	WATCHING_TYPE  = 3
	COMPETING_TYPE = 5
)

type DiscordHandshake struct {
	V        int    `json:"v"`
	ClientId string `json:"client_id"`
}

type DiscordHandshakeAsw struct {
	Cmd string `json:"cmd"`
	Evt string `json:"evt"`
}

type DiscordRPTimestamp struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type DiscordRPAssets struct {
	LargeImage string `json:"large_image"`
	LargeText  string `json:"large_text"`
	LargeUrl   string `json:"large_url"`
	SmallImage string `json:"small_image"`
	SmallText  string `json:"small_text"`
	SmallUrl   string `json:"small_url"`
}

type DiscordRPParty struct {
	Id   string `json:"id"`
	Size []int  `json:"size"`
}

type SetActivity struct {
	Cmd   string          `json:"cmd"`
	Nonce string          `json:"nonce"`
	Args  SetActivityArgs `json:"args"`
}

type SetActivityArgs struct {
	Pid      int                 `json:"pid"`
	Activity DiscordRichPresence `json:"activity"`
}

type DiscordRichPresence struct {
	Name       string             `json:"name"`
	Type       int                `json:"type"`
	Details    string             `json:"details"`
	State      string             `json:"state"`
	Party      DiscordRPParty     `json:"party"`
	Timeatamps DiscordRPTimestamp `json:"timeatamps"`
	Assets     DiscordRPAssets    `json:"assets"`
}

func (d DiscordHandshake) MarshalDiscordHandshake() ([]byte, error) {
	jsonData, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Discord handshake: %w", err)
	}

	payload := make([]byte, 0, 8+len(jsonData))

	payload = binary.LittleEndian.AppendUint32(payload, uint32(0))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(len(jsonData)))
	payload = append(payload, jsonData...)

	return payload, nil
}

func FindDiscordSocket() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/run/user/1000"
	}
	return runtimeDir + "/discord-ipc-0"
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	sock, err := net.Dial("unix", FindDiscordSocket())
	if err != nil {
		log.Fatalf("Failed to connect to Discord socket: %v", err)
	}
	defer sock.Close()

	log.Println("Connected to Discord socket")
	log.Println("Preparing Handshake")

	jsonPayload := DiscordHandshake{
		V:        1,
		ClientId: os.Getenv("DISCORD_CLIENT_ID"),
	}

	payload, err := jsonPayload.MarshalDiscordHandshake()
	if err != nil {
		log.Fatalf("Failed to marshal Discord handshake: %v", err)
	}

	_, err = sock.Write(payload)
	if err != nil {
		log.Fatalf("Failed to send Discord handshake: %v", err)
	}

	log.Println("Sent Discord handshake")

	for {
		header := make([]byte, 8)
		_, err := io.ReadFull(sock, header)
		if err != nil {
			log.Fatalf("Failed to read header from Discord socket: %v", err)
		}

		opcode := binary.LittleEndian.Uint32(header[0:4])
		lenJson := binary.LittleEndian.Uint32(header[4:8])

		jsonBuf := make([]byte, lenJson)
		_, err = io.ReadFull(sock, jsonBuf)
		if err != nil {
			log.Fatalf("Failed to read JSON payload from Discord socket: %v", err)
		}

		log.Printf("Received Opcode %d (%d bytes): %s", opcode, lenJson, string(jsonBuf))

		asw := DiscordHandshakeAsw{}
		err = json.Unmarshal(jsonBuf, &asw)
		if err != nil {
			log.Fatalf("Failed to unmarshal JSON payload: %v", err)
		}

		if asw.Cmd == "DISPATCH" && asw.Evt == "READY" {
			log.Println("Received READY event from Discord!")
			break
		} else {
			log.Printf("Received unexpected response (Opcode %d): %s", opcode, string(jsonBuf))
			return
		}
	}

	RP := DiscordRichPresence{
		Name:    "Test",
		Type:    5,
		Details: "test",
		State:   "In a Group",
		Assets: DiscordRPAssets{
			LargeImage: "https://logodownload.org/wp-content/uploads/2017/11/discord-logo-0-2048x2048.png",
			LargeText:  "Discord",
			LargeUrl:   "https://logodownload.org/wp-content/uploads/2017/11/discord-logo-0-2048x2048.png",
			SmallImage: "https://logodownload.org/wp-content/uploads/2017/11/discord-logo-0-2048x2048.png",
			SmallText:  "Discord",
			SmallUrl:   "https://logodownload.org/wp-content/uploads/2017/11/discord-logo-0-2048x2048.png",
		},
		Party: DiscordRPParty{
			Id:   "1234567890",
			Size: []int{1, 1},
		},
	}

	activity := SetActivity{
		Cmd:   "SET_ACTIVITY",
		Nonce: uuid.New().String(),
		Args:  SetActivityArgs{Pid: os.Getpid(), Activity: RP},
	}

	payloadJson, err := json.Marshal(activity)
	if err != nil {
		log.Fatalf("Failed to marshal Discord activity: %v", err)
	}

	payload = make([]byte, 0, 8+len(payloadJson))

	payload = binary.LittleEndian.AppendUint32(payload, uint32(1))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(len(payloadJson)))
	payload = append(payload, payloadJson...)

	_, err = sock.Write(payload)
	if err != nil {
		log.Fatalf("Failed to send Discord activity: %v", err)
	}

	for {
		header := make([]byte, 5000)

		n, err := sock.Read(header)
		if err != nil {
			log.Printf("Failed to read header from Discord socket: %v", err)
		}

		log.Printf("Received: %s", string(header[:n]))
	}

}
