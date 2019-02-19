package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// message sent to us by the javascript client
type message struct {
	Handle string `json:"handle"`
	Text   string `json:"text"`
}

// validateMessage so that we know it's valid JSON and contains a Handle and
// Text
func validateMessage(data []byte) (message, error) {
	var msg message

	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, errors.Wrap(err, "Unmarshaling message")
	}

	if msg.Handle == "" && msg.Text == "" {
		return msg, errors.New("Message has no Handle or Text")
	}

	return msg, nil
}

// handleWebsocket connection.
func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		m := "Unable to upgrade to websockets"
		log.WithField("err", err).Println(m)
		http.Error(w, m, http.StatusBadRequest)
		return
	}

	rr.register(ws)

	defer rr.deRegister(ws)
	defer ws.WriteMessage(websocket.CloseMessage, []byte{})
	defer ws.Close()

	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		mt, reader, err := ws.NextReader()
		//mt, data, err := ws.ReadMessage()
		l := log.WithFields(logrus.Fields{"mt": mt, "err": err})
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) || err == io.EOF {
				l.Info("Websocket closed!")
				break
			}
			l.Error(errors.Wrap(err, "Error reading websocket message"))
		}

		switch mt {
		case websocket.TextMessage:
			message, err := ioutil.ReadAll(reader)
			if err != nil {
				l.WithFields(logrus.Fields{"message": message, "err": err}).Error("Error reading message")
				break
			}
			msg, err := validateMessage(message)
			if err != nil {
				l.WithFields(logrus.Fields{"msg": msg, "err": err}).Error("Invalid Message")
				break
			}
			rw.publish(message)
		case websocket.PingMessage:
			l.Info("Ping received")
		default:
			l.Warning("Unknown Message!")
		}
	}


}
