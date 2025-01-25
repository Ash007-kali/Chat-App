package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Server struct {
	clients       map[*Client]*Client
	waitList      []*Client
	mu            sync.Mutex
	register      chan *Client
	unregister    chan *Client
	chatStartChan chan struct{}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func newServer() *Server {
	return &Server{
		clients:       make(map[*Client]*Client),
		waitList:      make([]*Client, 0),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		chatStartChan: make(chan struct{}),
	}
}

func (s *Server) run() {
	for {
		select {
		case client := <-s.register:
			s.handleNewClient(client)
		case client := <-s.unregister:
			s.handleClientDisconnect(client)
		}
	}
}

func (s *Server) handleNewClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.waitList) > 0 {
		// Pair with another client
		partner := s.waitList[0]
		s.waitList = s.waitList[1:]
		s.clients[client] = partner
		s.clients[partner] = client

		go s.startChat(client, partner)
	} else {
		// Add to the waitlist
		s.waitList = append(s.waitList, client)
	}
}

func (s *Server) cleanupWaitList(client *Client) {

	for i, c := range s.waitList {
		if c == client {
			fmt.Println("Disconnected a waiting client")
			s.waitList = append(s.waitList[:i], s.waitList[i+1:]...)
			return
		}
	}
}

func (s *Server) handleClientDisconnect(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupWaitList(client)

	if partner, ok := s.clients[client]; ok {
		delete(s.clients, client)
		delete(s.clients, partner)
		partner.conn.WriteMessage(websocket.TextMessage, []byte("Partner disconnected"))
		partner.conn.Close()
	}

	client.conn.Close()
}

func (s *Server) startChat(client1, client2 *Client) {
	client1.conn.WriteMessage(websocket.TextMessage, []byte("connected"))
	client2.conn.WriteMessage(websocket.TextMessage, []byte("connected"))

	go s.handleMessages(client1, client2)
	go s.handleMessages(client2, client1)
}

func (s *Server) handleMessages(sender, receiver *Client) {
	for {
		_, message, err := sender.conn.ReadMessage()
		if err != nil {
			s.unregister <- sender
			return
		}
		receiver.conn.WriteMessage(websocket.TextMessage, message)
	}
}

func serveWs(s *Server, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade error:", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte),
	}

	conn.SetCloseHandler(func(code int, text string) error {
		s.unregister <- client
		return nil
	})

	s.register <- client
}

func main() {
	s := newServer()
	go s.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(s, w, r)
	})

	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
