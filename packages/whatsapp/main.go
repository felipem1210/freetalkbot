package whatsapp

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

var (
	whatsappClient    *whatsmeow.Client
	openaiClient      openai.Client
	language          string
	assistantLanguage string
	jid               string
	err               error
)

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
	jid = parseJid(v.Info.Sender.String())
	assistantLanguage = os.Getenv("ASSISTANT_LANGUAGE")

	if messageBody != "" {
		//var translation string
		slog.Info("Received message", "jid", jid)

		language, err = openai.DetectLanguage(openaiClient, messageBody)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to detect language: %v", err), "jid", jid)
			return
		} else {
			slog.Debug(fmt.Sprintf("detected language: %s", language), "jid", jid)
		}

		if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
			messageBody, err = openai.TranslateText(openaiClient, messageBody, assistantLanguage)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to translate messageBody: %v", err), "jid", jid)
				return
			} else {
				slog.Debug(fmt.Sprintf("translated messageBody: %s", messageBody), "jid", jid)
			}
		}

		rasaUri := rasa.ChooseUri(messageBody)
		respBody, err := rasa.SendMessage(rasaUri, jid, messageBody)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending message: %s", err), "jid", jid)
			return
		}
		if rasaUri == "webhooks/rest/webhook" {
			responses, err := rasa.HandleResponseBody(respBody)
			if err != nil {
				slog.Error(fmt.Sprintf("Error handling response body: %s", err), "jid", jid)
				return
			}
			for _, response := range responses {
				if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
					response.Text, err = openai.TranslateText(openaiClient, response.Text, language)
					if err != nil {
						slog.Error(fmt.Sprintf("failed to translate response: %v", err), "jid", jid)
						return
					} else {
						slog.Debug(fmt.Sprintf("translated response: %s", response.Text), "jid", jid)
					}
				}
				result, error := sendWhatsappResponse(jid, &response)
				if error != nil {
					slog.Error(fmt.Sprintf("Error sending response: %s", error), "jid", jid)
					return
				} else {
					slog.Info(result, "jid", jid)
				}
			}
		}
	}

	if audioMessage := v.Message.GetAudioMessage(); audioMessage != nil {
		transcription, err := transcribeAudio(audioMessage, v.Info.ID)
		if err != nil {
			slog.Error(fmt.Sprintf("Error transcribing audio message: %s", err), "jid", jid)
			return
		}

		language, err = openai.DetectLanguage(openaiClient, transcription)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to detect language: %v", err), "jid", jid)
			return
		} else {
			slog.Debug(fmt.Sprintf("detected language: %s", language), "jid", jid)
		}

		if !strings.Contains(language, assistantLanguage) && assistantLanguage != language {
			transcription, err = openai.TranslateText(openaiClient, transcription, assistantLanguage)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to translate transcription: %v", err), "jid", jid)
				return
			} else {
				slog.Debug(fmt.Sprintf("translated transcription: %s", messageBody), "jid", jid)
			}
		}

		rasaUri := rasa.ChooseUri(transcription)
		slog.Debug(fmt.Sprintf("rasa uri: %s", rasaUri), "jid", jid)

		_, err = rasa.SendMessage(rasaUri, jid, transcription)
		if err != nil {
			slog.Error(fmt.Sprintf("Error sending message to rasa: %s", err), "jid", jid)
			return
		}
	}
}

func parseJid(jid string) string {
	// Check if the JID is in the format phone_number@domain
	// If is in format phone_number:device_id@domain, remove the device_id
	if len(strings.Split(strings.Split(jid, "@")[0], ":")) == 2 {
		fmt.Printf("JID: %s\n", jid)
		jid = fmt.Sprintf("%s@%s", strings.Split(strings.Split(jid, "@")[0], ":")[0], strings.Split(jid, "@")[1])
	}
	return jid
}

func sendWhatsappResponse(jidStr string, response *rasa.Response) (string, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %v", jidStr)
	}
	_, err = whatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
		Conversation: proto.String(response.Text),
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
	transcription, err := openai.TranscribeAudio(openaiClient, audioFilePath)
	if err != nil {
		return "", err
	}
	return transcription, nil
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
		slog.Error(fmt.Sprintf("Failed to initialize SQL store: %v", err))
		os.Exit(1)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get device: %v", err))
		os.Exit(1)
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
