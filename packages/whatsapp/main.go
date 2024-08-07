package whatsapp

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/felipem1210/freetalkbot/packages/common"
	openai "github.com/felipem1210/freetalkbot/packages/openai"
	rasa "github.com/felipem1210/freetalkbot/packages/rasa"
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
		translation, _ := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["translation_english"], messageBody))
		language, _ = openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["language"], messageBody))
		rasaUri := rasa.ChooseUri(translation)
		respBody := rasa.SendMessage(rasaUri, jid, translation)
		if rasaUri == "webhooks/rest/webhook" {
			responses := rasa.HandleResponseBody(respBody)
			for _, response := range responses {
				responseTranslated, _ := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["translation"], response.Text, language))
				response.Text = responseTranslated
				_ = sendWhatsappResponse(jid, &response)
			}
		}
	}

	if audioMessage := v.Message.GetAudioMessage(); audioMessage != nil {
		transcription, translation, err := handleAudioMessage(audioMessage, v.Info.ID)
		rasaUri := rasa.ChooseUri(translation)
		language, _ = openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["language"], transcription))
		if err != nil {
			log.Printf("Error handling audio message: %s", err)
			return
		}

		_ = rasa.SendMessage(rasaUri, jid, translation)
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
	if err := downloadAudio(audioMessage.GetURL(), common.AudioEncPath); err != nil {
		return "", "", err
	}
	audioFilePath := fmt.Sprintf("%s%s.ogg", common.AudioDir, messageId)
	if err := decryptAudioFile(common.AudioEncPath, audioFilePath, mediaKeyHex); err != nil {
		return "", "", err
	}
	transcription, err := openai.TranscribeAudio(openaiClient, audioFilePath)
	if err != nil {
		return "", "", err
	}
	translation, err := openai.ConsultChatGpt(openaiClient, fmt.Sprintf(common.ChatgptQueries["translation_english"], transcription))
	if err != nil {
		return "", "", err
	}
	return transcription, translation, nil
}

func decryptAudioFile(inputFilePath, outputFilePath, mediaKey string) error {
	cmdString := fmt.Sprintf("whatsapp-media-decrypt -o %s -t 3 %s %s", outputFilePath, inputFilePath, mediaKey)
	err := common.ExecuteCommand(cmdString)
	if err != nil {
		return err
	}
	return nil
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
