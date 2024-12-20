package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

// const (
// 	// Time allowed to write a message to the peer.
// 	writeWait = 10 * time.Second

// 	// Time allowed to read the next pong message from the peer.
// 	pongWait = 60 * time.Second

// 	// Send pings to peer with this period. Must be less than pongWait.
// 	pingPeriod = (pongWait * 9) / 10

// 	// Maximum message size allowed from peer.
// 	maxMessageSize = 512
// )

// var (
// 	newline = []byte{'\n'}
// 	space   = []byte{' '}
// )

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Room struct {
	id      string
	player1 *websocket.Conn
	player2 *websocket.Conn
	board   [3][3]string
	turn    string // x or o
	state   string // waiting, in-progress, finished
}

type Games struct {
	rooms map[string]*Room
}

var games = Games{
	rooms: make(map[string]*Room),
}

// // Client is a middleman between the websocket connection and the hub.
// type Client struct {
// 	hub *Hub

// 	// The websocket connection.
// 	conn *websocket.Conn

// 	// Buffered channel of outbound messages.
// 	send chan []byte
// }

// // readPump pumps messages from the websocket connection to the hub.
// //
// // The application runs readPump in a per-connection goroutine. The application
// // ensures that there is at most one reader on a connection by executing all
// // reads from this goroutine.
// func (c *Client) readPump() {
// 	defer func() {
// 		c.hub.unregister <- c
// 		c.conn.Close()
// 	}()
// 	c.conn.SetReadLimit(maxMessageSize)
// 	c.conn.SetReadDeadline(time.Now().Add(pongWait))
// 	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
// 	for {
// 		_, message, err := c.conn.ReadMessage()
// 		if err != nil {
// 			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
// 				log.Printf("error: %v", err)
// 			}
// 			break
// 		}
// 		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
// 		c.hub.broadcast <- message
// 	}
// }

// // writePump pumps messages from the hub to the websocket connection.
// //
// // A goroutine running writePump is started for each connection. The
// // application ensures that there is at most one writer to a connection by
// // executing all writes from this goroutine.
// func (c *Client) writePump() {
// 	ticker := time.NewTicker(pingPeriod)
// 	defer func() {
// 		ticker.Stop()
// 		c.conn.Close()
// 	}()
// 	for {
// 		select {
// 		case message, ok := <-c.send:
// 			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
// 			if !ok {
// 				// The hub closed the channel.
// 				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
// 				return
// 			}

// 			w, err := c.conn.NextWriter(websocket.TextMessage)
// 			if err != nil {
// 				return
// 			}
// 			w.Write(message)

// 			// Add queued chat messages to the current websocket message.
// 			n := len(c.send)
// 			for i := 0; i < n; i++ {
// 				w.Write(newline)
// 				w.Write(<-c.send)
// 			}

// 			if err := w.Close(); err != nil {
// 				return
// 			}
// 		case <-ticker.C:
// 			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
// 			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
// 				return
// 			}
// 		}
// 	}
// }

func serveWs(w http.ResponseWriter, r *http.Request, roomId string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	if room, ok := games.rooms[roomId]; ok {
		fmt.Println("room already exists:", roomId)

		room.player2 = conn
		room.state = "in-progress"

		fmt.Println("starting game for room:", roomId)
		go handleGame(room, conn)
	} else {
		fmt.Println("no room found, creating new one:", roomId)

		room := Room{
			id:      roomId,
			player1: conn,
			turn:    "x",
			state:   "waiting",
		}
		games.rooms[roomId] = &room

		fmt.Println("waiting for player 2 for room:", roomId)
		go handleGame(&room, conn)
	}
}

func handleGame(room *Room, conn *websocket.Conn) {
	for {
		var move string
		if err := conn.ReadJSON(&move); err != nil {
			break
		}

		fmt.Printf("move: %s\t for room: %s\n", move, room.id)
		// a move is 3 character string: 11x, 01o
		s := strings.Split(move, "")
		x, _ := strconv.Atoi(s[0])
		y, _ := strconv.Atoi(s[1])
		turn := s[2]
		room.board[x][y] = turn
	}
}

type model struct {
	width   int
	height  int
	posX    int
	posY    int
	canMove bool
}

func initServer() {
	args := os.Args[1:]

	roomId := ""
	if len(args) > 0 {
		roomId = args[0]
	}

	fmt.Println("roomId", roomId)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		roomId := r.URL.Query().Get("room")
		serveWs(w, r, roomId)
	})
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("websocket:", err)
	}
}

func (m model) Init() tea.Cmd {
	go initServer()
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if s := msg.String(); s == "ctrl+c" || s == "q" || s == "esc" {
			return m, tea.Quit
		}
		if s := msg.String(); s == "h" || s == "left" {
			if m.posX > 0 {
				m.posX -= 1
			}
		} else if s == "j" || s == "down" {
			if m.posY < 2 {
				m.posY += 1
			}
		} else if s == "l" || s == "right" {
			if m.posX < 2 {
				m.posX += 1
			}
		} else if s == "k" || s == "up" {
			if m.posY > 0 {
				m.posY -= 1
			}
		}

		if s := msg.String(); s == "space" {
			if m.canMove {
				// TODO:
				// games[]
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m model) View() string {
	s := ""

	for i := 0; i < 3; i++ {
		var rows [3]string
		for j := 0; j < 3; j++ {
			borderColor := lipgloss.Color("225")
			if m.posX == j && m.posY == i {
				borderColor = lipgloss.Color("212")
			}
			rows[j] = lipgloss.NewStyle().
				Align(lipgloss.Center, lipgloss.Center).
				Border(lipgloss.NormalBorder()).
				BorderForeground(borderColor).
				Width(m.width/3 - 28).
				Height(m.height/3 - 3).
				Render(fmt.Sprintf("%d%d", i, j))
		}
		s += lipgloss.JoinHorizontal(lipgloss.Center, rows[0], rows[1], rows[2]) + "\n"
	}

	s += "h,j,k,l or arrow keys to move around\n"
	s += "Space to mark"

	return s
}

func main() {
	p := tea.NewProgram(model{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
