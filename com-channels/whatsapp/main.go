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

func GetEventHandler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			var messageId = v.Info.ID
			var messageBody = v.Message.GetConversation()
			var senderName = v.Info.Sender.String()
			fmt.Printf("Message from %s: %s\n", senderName, messageBody)
			if messageBody != "" {
				// send http request to rasa server
				respBody := rasa.SendMessage("webhooks/rest/webhook", messageBody)
				responses := rasa.ReceiveMessage(respBody)
				sendWhatsappResponse(responses, client, v)
			}
			if v.Message.GetAudioMessage() != nil {
				var callbackResponses []rasa.Response
				audioMessage := v.Message.GetAudioMessage()
				translation, err := handleAudioMessage(audioMessage, messageId)
				if err != nil {
					log.Fatalf("Error handling audio message: %s", err)
				}
				_ = rasa.SendMessage("webhooks/callback/webhook", translation)
				callbackResponse := &rasa.CallbackResponse
				callbackResponses = append(callbackResponses, *callbackResponse)
				fmt.Printf("Callback response: %v\n", callbackResponses)
				sendWhatsappResponse(callbackResponses, client, v)

			}
		}
	}
}

func sendWhatsappResponse(responses []rasa.Response, client *whatsmeow.Client, v *events.Message) {
	var finalResponse string
	for _, response := range responses {
		if response.Image != "" && response.Image != "<nil>" {
			finalResponse = response.Image
		} else {
			finalResponse = response.Text
		}
		client.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
			Conversation: proto.String(finalResponse),
		})
	}
}

func handleAudioMessage(audioMessage *waE2E.AudioMessage, messageId string) (string, error) {
	mediaKeyHex := hex.EncodeToString(audioMessage.GetMediaKey())
	err := downloadAudio(audioMessage.GetURL(), "audios/audio.enc")
	if err != nil {
		return "", err
	}
	err = decryptAudioFile("audios/audio.enc", "audios/"+messageId+".ogg", mediaKeyHex)
	if err != nil {
		return "", err
	}
	openAiClient := openai.CreateOpenAiClient()
	transcription, err := openai.TranscribeAudio(openAiClient, "audios/"+messageId+".ogg")
	if err != nil {
		return "", err
	}
	translation, err := openai.TranslateText(openAiClient, transcription)
	if err != nil {
		return "", err
	}
	return translation, nil
}

func decryptAudioFile(inputFilePath string, outputFilePath string, mediaKey string) error {
	// Execute whatsapp-media-decrypt tool
	cmdString := fmt.Sprintf("whatsapp-media-decrypt -o %s -t 3 %s %s", outputFilePath, inputFilePath, mediaKey)
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	f, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, f)
	return nil
}

func downloadAudio(url, dest string) error {
	// Create the destination file
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Make the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the HTTP status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Copy the content to the file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func InitializeServer() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %s", err)
	}
	sqlDbFileName := os.Getenv("SQL_DB_FILE_NAME") // examplestore.db
	dbLog := waLog.Stdout("Database", "INFO", true)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite
	container, err := sqlstore.New("sqlite3", "file:"+sqlDbFileName+"?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(GetEventHandler(client))

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		pairPhoneNumber, varExists := os.LookupEnv("PAIR_PHONE_NUMBER")
		if varExists {
			code, err := client.PairPhone(pairPhoneNumber, true, whatsmeow.PairClientChrome, "Chrome (MacOS)")
			if err != nil {
				panic(err)
			}
			fmt.Println("Pairing code:", code)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
