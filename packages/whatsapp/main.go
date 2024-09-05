package whatsapp

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	gt "github.com/bas24/googletranslatefree"
	"github.com/felipem1210/freetalkbot/packages/common"
	"github.com/felipem1210/freetalkbot/packages/rasa"
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

var (
	whatsappClient    *whatsmeow.Client
	openaiClient      common.OpenaiClient
	language          string
	assistantLanguage string
	transcription     string
	jid               string
	err               error
)

func getEventHandler() func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			handleMessageEvent(v)
		}
	}
}

func handleMessageEvent(v *events.Message) {
	messageBody := v.Message.GetConversation()
	jid = parseJid(v.Info.Sender.String())
	assistantLanguage = os.Getenv("ASSISTANT_LANGUAGE")

	if messageBody != "" {
		slog.Info("Received text message", "jid", jid)
	} else if audioMessage := v.Message.GetAudioMessage(); audioMessage != nil {
		slog.Info("Received audio message", "jid", jid)
		transcription, err = transcribeAudio(audioMessage, v.Info.ID)
		messageBody = transcription
		if err != nil {
			slog.Error(fmt.Sprintf("Error transcribing audio message: %s", err), "jid", jid)
			return
		}
	}
	slog.Debug(fmt.Sprintf("message received: %s", messageBody), "jid", jid)

	language = common.DetectLanguage(messageBody)
	slog.Debug(fmt.Sprintf("detected language: %s", language), "jid", jid)

	if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
		messageBody, _ = gt.Translate(messageBody, language, assistantLanguage)
		slog.Debug(fmt.Sprintf("translated message: %s", messageBody), "jid", jid)
	}

	rasaHandler := rasa.Rasa{
		MessageLanguage: language,
		RasaLanguage:    assistantLanguage,
	}

	rasaHandler.Request.JsonBody = map[string]string{"sender": jid, "message": messageBody}
	response, err := rasaHandler.Interact()
	if err != nil {
		slog.Error(fmt.Sprintf("Error interacting with Rasa: %s", err), "jid", jid)
		return
	}
	slog.Debug(fmt.Sprintf("response from rasa: %v", response), "jid", jid)
	handleResponses(response)
}

func handleResponses(response common.Response) {
	for _, r := range response.RasaResponse {
		result, error := sendWhatsappMessage(r.RecipientId, r.Text)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending response: %s", error), "jid", jid)
		} else {
			slog.Info(result, "jid", r.RecipientId)
		}
	}
}

func sendWhatsappMessage(jidStr string, message string) (string, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %v", jidStr)
	}
	_, err = whatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
		Conversation: proto.String(message),
	})
	if err != nil {
		return "", fmt.Errorf("failed to send message: %v", err)

	}
	return fmt.Sprintf("Message sent to %s", jidStr), nil
}

func transcribeAudio(audioMessage *waE2E.AudioMessage, messageId string) (string, error) {
	mediaKeyHex := hex.EncodeToString(audioMessage.GetMediaKey())
	if err := downloadAudio(audioMessage.GetURL(), common.AudioEncPath); err != nil {
		return "", err
	}
	audioFilePath := fmt.Sprintf("%s%s.ogg", common.AudioDir, messageId)
	if err := decryptAudioFile(common.AudioEncPath, audioFilePath, mediaKeyHex); err != nil {
		return "", err
	}

	transcription, err = common.TranscribeAudio(audioFilePath, openaiClient)
	slog.Debug(fmt.Sprintf("transcription: %s", transcription), "jid", jid)

	return transcription, nil
}

func InitializeServer() {
	sqlDbFileName := os.Getenv("SQL_DB_FILE_NAME")
	dbLog := waLog.Stdout("Database", "INFO", true)

	container, err := sqlstore.New("sqlite3", "file:"+sqlDbFileName+"?_foreign_keys=on", dbLog)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize SQL store: %v", err))
		os.Exit(1)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get device: %v", err))
		os.Exit(1)
	}

	if os.Getenv("STT_TOOL") == "whisper" {
		openaiClient = common.CreateOpenAiClient()
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	whatsappClient = whatsmeow.NewClient(deviceStore, clientLog)
	whatsappClient.AddEventHandler(getEventHandler())
	handleClientConnection(whatsappClient)
}

func handleClientConnection(client *whatsmeow.Client) {
	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		if err := client.Connect(); err != nil {
			slog.Error(fmt.Sprintf("Failed to connect: %v", err))
			os.Exit(1)
		}
		pairPhoneNumber, varExists := os.LookupEnv("PAIR_PHONE_NUMBER")
		if varExists {
			if code, err := client.PairPhone(pairPhoneNumber, true, whatsmeow.PairClientChrome, "Chrome (MacOS)"); err != nil {
				slog.Error(fmt.Sprintf("Failed to pair phone: %v", err))
				os.Exit(1)
			} else {
				slog.Info(fmt.Sprintf("Pairing code: %s", code))
			}
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				slog.Info(fmt.Sprintf("QR code: %s", evt.Code))
			} else {
				slog.Info(fmt.Sprintf("Login event: %s", evt.Event))
			}
		}
	} else {
		if err := client.Connect(); err != nil {
			slog.Error(fmt.Sprintf("Failed to connect: %v", err))
			os.Exit(1)
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
