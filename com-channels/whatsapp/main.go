package whatsapp

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	openai "github.com/felipem1210/freetalkbot/genai/openai"
	rasa "github.com/felipem1210/freetalkbot/rasa"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const (
	audioEncPath = "audios/audio.enc"
	audioDir     = "audios/"
)

var (
	chatgptQueries = map[string]string{
		"translation_english": "Hello, translate this %s to english, if it is already in english, do nothing",
		"language":            "Hello, please identify the language of this text: %s. Give me only the language name",
		"translation":         "Hello, translate this %s to this language %s.",
	}
)

var whatsappClient *whatsmeow.Client
var openaiClient openai.Client
var language string

func GetEventHandler() func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			handleMessageEvent(v)
		}
	}
}

func handleMessageEvent(v *events.Message) {
	messageBody := v.Message.GetConversation()
	jid := v.Info.Sender.String()

	if messageBody != "" {
		log.Printf("Message from %s\n", jid)
		translation, _ := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(chatgptQueries["translation_english"], messageBody))
		respBody := rasa.SendMessage("webhooks/rest/webhook", jid, translation)
		responses := rasa.HandleResponseBody(respBody)
		language, _ = openai.ConsultChatGpt(openaiClient, fmt.Sprintf(chatgptQueries["language"], messageBody))
		for _, response := range responses {
			responseTranslated, _ := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(chatgptQueries["translation"], response.Text, language))
			response.Text = responseTranslated
			sucess := sendWhatsappResponse(jid, &response)
			log.Println(sucess)
		}

	}

	if audioMessage := v.Message.GetAudioMessage(); audioMessage != nil {
		transcription, translation, err := handleAudioMessage(audioMessage, v.Info.ID)
		language, _ = openai.ConsultChatGpt(openaiClient, fmt.Sprintf(chatgptQueries["language"], transcription))
		if err != nil {
			log.Printf("Error handling audio message: %s", err)
			return
		}

		_ = rasa.SendMessage("webhooks/callback/webhook", jid, translation)
	}
}

func sendWhatsappResponse(to string, response *rasa.Response) string {
	jid, err := types.ParseJID(to)
	if err != nil {
		log.Fatalf("Invalid JID: %v", to)
	}
	_, err = whatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
		Conversation: proto.String(response.Text),
	})
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}
	return fmt.Sprintf("Message sent to %s", to)
}

func handleAudioMessage(audioMessage *waE2E.AudioMessage, messageId string) (string, string, error) {
	mediaKeyHex := hex.EncodeToString(audioMessage.GetMediaKey())
	if err := downloadAudio(audioMessage.GetURL(), audioEncPath); err != nil {
		return "", "", err
	}
	audioFilePath := fmt.Sprintf("%s%s.ogg", audioDir, messageId)
	if err := decryptAudioFile(audioEncPath, audioFilePath, mediaKeyHex); err != nil {
		return "", "", err
	}
	transcription, err := openai.TranscribeAudio(openaiClient, audioFilePath)
	if err != nil {
		return "", "", err
	}
	translation, err := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(chatgptQueries["translation_english"], transcription))
	if err != nil {
		return "", "", err
	}
	return transcription, translation, nil
}

func decryptAudioFile(inputFilePath, outputFilePath, mediaKey string) error {
	cmdString := fmt.Sprintf("whatsapp-media-decrypt -o %s -t 3 %s %s", outputFilePath, inputFilePath, mediaKey)
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	f, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return err
}

func downloadAudio(url, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func InitializeServer() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}
	sqlDbFileName := os.Getenv("SQL_DB_FILE_NAME")
	dbLog := waLog.Stdout("Database", "INFO", true)

	container, err := sqlstore.New("sqlite3", "file:"+sqlDbFileName+"?_foreign_keys=on", dbLog)
	if err != nil {
		log.Fatalf("Failed to initialize SQL store: %v", err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		log.Fatalf("Failed to get device: %v", err)
	}

	openaiClient = openai.CreateNewClient()

	clientLog := waLog.Stdout("Client", "INFO", true)
	whatsappClient = whatsmeow.NewClient(deviceStore, clientLog)
	whatsappClient.AddEventHandler(GetEventHandler())
	handleClientConnection(whatsappClient)
}

func handleClientConnection(client *whatsmeow.Client) {
	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		if err := client.Connect(); err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
		pairPhoneNumber, varExists := os.LookupEnv("PAIR_PHONE_NUMBER")
		if varExists {
			if code, err := client.PairPhone(pairPhoneNumber, true, whatsmeow.PairClientChrome, "Chrome (MacOS)"); err != nil {
				log.Fatalf("Failed to pair phone: %v", err)
			} else {
				fmt.Println("Pairing code:", code)
			}
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		if err := client.Connect(); err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}
	}
	waitForShutdown(client)
}

func waitForShutdown(client *whatsmeow.Client) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	client.Disconnect()
}
