package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// Define a structure to match the JSON response
type Response struct {
	RecipientID string `json:"recepient_id"`
	Text        string `json:"text"`
	Image       string `json:"image"`
}

func GetEventHandler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			var messageBody = v.Message.GetConversation()
			var senderName = v.Info.Sender.String()
			fmt.Printf("Message from %s: %s\n", senderName, messageBody)
			if messageBody != "" {
				// send http request to rasa server
				responses := sendHttpRequest(messageBody)
				var finalResponse string
				for _, response := range responses {
					if response.Image != "" {
						finalResponse = response.Image
					} else {
						finalResponse = response.Text
					}
					client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						Conversation: proto.String(finalResponse),
					})
				}
			}
		}
	}
}

// send POST http request to a server, the content will be in json format
func sendHttpRequest(m string) []Response {
	rasaUrl := os.Getenv("RASA_URL") // http://localhost:5005/webhooks/rest/webhook
	// Data to be sent in the request body
	data := map[string]string{
		"sender":  "sender",
		"message": m,
	}

	// Convert the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error converting data to JSON: %s", err)
	}

	// Create the POST request
	req, err := http.NewRequest("POST", rasaUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Error creating request: %s", err)
	}

	// Set the content type to JSON
	req.Header.Set("Content-Type", "application/json")

	// Create an HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %s", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response: %s", err)
	}
	fmt.Printf("Response: %s\n", body)

	// Parse the JSON response
	var responses []Response
	//var finalResponse string
	err = json.Unmarshal(body, &responses)
	if err != nil {
		log.Fatalf("Error parsing JSON response: %s", err)
	}
	return responses
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
