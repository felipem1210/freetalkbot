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
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const (
	audioEncPath = "audios/audio.enc"
	audioDir     = "audios/"
)

func GetEventHandler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			handleMessageEvent(v, client)
		}
	}
}

func handleMessageEvent(v *events.Message, client *whatsmeow.Client) {
	messageBody := v.Message.GetConversation()
	senderName := v.Info.Sender.String()
	fmt.Printf("Message from %s: %s\n", senderName, messageBody)

	if messageBody != "" {
		respBody := rasa.SendMessage("webhooks/rest/webhook", messageBody)
		responses := rasa.ReceiveMessage(respBody)
		if responses != nil {
			sendWhatsappResponse(responses, client, v)
		} else {
			log.Println("No valid responses received from Rasa")
		}
	}
	if audioMessage := v.Message.GetAudioMessage(); audioMessage != nil {
		var callbackResponses []rasa.Response
		translation, err := handleAudioMessage(audioMessage, v.Info.ID)
		if err != nil {
			log.Printf("Error handling audio message: %s", err)
			return
		}
		_ = rasa.SendMessage("webhooks/callback/webhook", translation)
		callbackResponse := &rasa.CallbackResponse
		callbackResponses = append(callbackResponses, *callbackResponse)
		fmt.Printf("Callback response: %v\n", callbackResponses)
		if callbackResponses != nil {
			sendWhatsappResponse(callbackResponses, client, v)
		} else {
			log.Println("No valid callback responses received from Rasa")
		}
	}
}

func sendWhatsappResponse(responses []rasa.Response, client *whatsmeow.Client, v *events.Message) {
	for _, response := range responses {
		finalResponse := response.Text
		if response.Image != "" && response.Image != "<nil>" {
			finalResponse = response.Image
		}
		client.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
			Conversation: proto.String(finalResponse),
		})
	}
}

func handleAudioMessage(audioMessage *waE2E.AudioMessage, messageId string) (string, error) {
	mediaKeyHex := hex.EncodeToString(audioMessage.GetMediaKey())
	if err := downloadAudio(audioMessage.GetURL(), audioEncPath); err != nil {
		return "", err
	}
	audioFilePath := fmt.Sprintf("%s%s.ogg", audioDir, messageId)
	if err := decryptAudioFile(audioEncPath, audioFilePath, mediaKeyHex); err != nil {
		return "", err
	}
	openAiClient := openai.CreateOpenAiClient()
	transcription, err := openai.TranscribeAudio(openAiClient, audioFilePath)
	if err != nil {
		return "", err
	}
	return openai.TranslateText(openAiClient, transcription)
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

	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(GetEventHandler(client))

	handleClientConnection(client)
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
