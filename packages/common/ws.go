package common

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type WsReq struct {
	Url  string
	Data []byte
}

type WsResponse struct {
	Text string `json:"text"`
}

// SendWsMessage sends a message to the websocket server
func (r *WsReq) SendWsMessage() (string, error) {
	c, _, err := websocket.DefaultDialer.Dial(r.Url, nil)
	if err != nil {
		return "", err
	}
	defer c.Close()

	// Enviar un mensaje binario (por ejemplo, un timestamp convertido en bytes)
	err = c.WriteMessage(websocket.BinaryMessage, r.Data)
	if err != nil {
		return "", err
	}

	// Goroutine to receive messages from the server
	_, message, err := c.ReadMessage()
	if err != nil {
		return "", err
	}

	var wsResp WsResponse
	err = json.Unmarshal(message, &wsResp)
	if err != nil {
		log.Println("Error al deserializar el JSON:", err)
		return "", err
	}

	return wsResp.Text, nil
}
