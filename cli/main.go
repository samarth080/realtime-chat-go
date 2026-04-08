package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var serverURL = flag.String("server", "http://localhost:8080", "server base URL")

func main() {
	flag.Parse()

	fmt.Println("P2P Chat CLI")
	fmt.Println("Commands: register <user> <pass> | login <user> <pass> | send <user_id> <msg> | ping | quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	var token string
	var ws *websocket.Conn

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		cmd := parts[0]

		switch cmd {
		case "register":
			if len(parts) < 3 {
				fmt.Println("usage: register <username> <password>")
				continue
			}
			t, err := authRequest(*serverURL+"/auth/register", parts[1], parts[2])
			if err != nil {
				fmt.Println("error:", err)
				continue
			}
			token = t
			fmt.Println("registered and logged in. token saved.")

		case "login":
			if len(parts) < 3 {
				fmt.Println("usage: login <username> <password>")
				continue
			}
			t, err := authRequest(*serverURL+"/auth/login", parts[1], parts[2])
			if err != nil {
				fmt.Println("error:", err)
				continue
			}
			token = t
			var err2 error
			ws, err2 = connectWS(*serverURL, token)
			if err2 != nil {
				fmt.Println("websocket connect error:", err2)
				continue
			}
			go receiveMessages(ws)
			fmt.Println("connected.")

		case "send":
			if len(parts) < 3 {
				fmt.Println("usage: send <user_id> <message>")
				continue
			}
			if ws == nil {
				fmt.Println("not connected. run login first.")
				continue
			}
			msg := map[string]string{
				"type": "message",
				"to":   parts[1],
				"body": parts[2],
				"id":   fmt.Sprintf("cli-%d", os.Getpid()),
			}
			data, _ := json.Marshal(msg)
			if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
				fmt.Println("send error:", err)
			}

		case "ping":
			if ws == nil {
				fmt.Println("not connected.")
				continue
			}
			ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))

		case "quit", "exit":
			if ws != nil {
				ws.Close()
			}
			fmt.Println("bye.")
			return

		default:
			fmt.Println("unknown command:", cmd)
		}
	}
}

func authRequest(url, username, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if errMsg, ok := result["error"]; ok {
		return "", fmt.Errorf("%v", errMsg)
	}
	token, ok := result["token"].(string)
	if !ok {
		return "", fmt.Errorf("no token in response")
	}
	return token, nil
}

func connectWS(serverURL, token string) (*websocket.Conn, error) {
	wsURL := strings.Replace(serverURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/ws?token=" + token

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	return conn, err
}

func receiveMessages(ws *websocket.Conn) {
	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			fmt.Println("\n[disconnected]")
			return
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg["type"] {
		case "message":
			fmt.Printf("\n[msg from %s]: %s\n> ", msg["from"], msg["body"])
		case "pong":
			fmt.Print("pong\n> ")
		case "sent":
			fmt.Printf("[ack: message %s sent]\n> ", msg["message_id"])
		case "typing":
			fmt.Printf("[%s is typing...]\n> ", msg["from"])
		}
	}
}
