package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Users struct {
	All        map[string]net.Conn
	WelcomePig string
	mu         sync.Mutex
}

func (u *Users) Add(conn net.Conn, name string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if _, ok := u.All[name]; ok {
		return errors.New("user already exists")
	}

	u.All[name] = conn
	return nil
}

func (u *Users) glist(conn net.Conn, name string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !ValidName(name, conn) {
		return errors.New("Incorect input name")
	}

	u.All[name] = conn
	return nil
}

func (u *Users) CheckPull() bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.All) > 10 {
		return false
	}

	return true
}

func (u *Users) Del(name string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	delete(u.All, name)
}

type History struct {
	Content string
	mu      sync.Mutex
}

func (h *History) Add(mess string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Content += mess
}

func (h *History) Get() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Content
}

type Message struct {
	Author string
	Msg    string
	time   time.Time
	conn   net.Conn
}

func (m Message) PreScan(conn net.Conn, name string) {
	t := m.time.Format("2006-01-02 15:04:05")
	fmt.Fprintf(conn, "[%s][%s]:", t, name)
}

func (m Message) String() string {
	t := m.time.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("\n[%s][%s]:%s\n", t, m.Author, m.Msg)
}

func (m Message) HistoryString() string {
	t := m.time.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("[%s][%s]:%s\n", t, m.Author, m.Msg)
}

const (
	whatYourName = "[ENTER YOUR NAME]:"
	joinChat     = " has joined our chat..."
	leftChat     = " has left our chat..."
)

var (
	port   string
	mess   chan Message = make(chan Message)
	status chan Message = make(chan Message)
)

func main() {
	flag.StringVar(&port, "port", "8989", "PORT DLIA NET-CAT")

	flag.Parse()
	if len(os.Args[1:]) > 1 {
		fmt.Println("too many argyment")
		return
	}
	pig, err := os.ReadFile("pig.txt")
	if err != nil {
		fmt.Println(err)
		return
	}

	users := Users{
		All:        map[string]net.Conn{},
		WelcomePig: string(pig),
		mu:         sync.Mutex{},
	}

	history := History{
		mu: sync.Mutex{},
	}

	go BroadCaster(&users, &history)

	listener, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("starts listen localhost:" + port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(conn, err)
			conn.Close()
			continue
		}

		go Client(conn, &users, &history)
	}
}

func BroadCaster(u *Users, h *History) {
	for {
		select {
		case msg := <-mess:
			h.Add(msg.HistoryString())
			u.mu.Lock()
			for name, conn := range u.All {
				if name != msg.Author {
					fmt.Fprint(conn, msg)
				}
				msg.PreScan(conn, name)
			}
			u.mu.Unlock()

		case stat := <-status:
			h.Add(stat.Author + stat.Msg + "\n")
			u.mu.Lock()
			for name, conn := range u.All {
				if name != stat.Author {
					fmt.Fprint(conn, "\n", stat.Author+stat.Msg, "\n")
					stat.PreScan(conn, name)
				}
			}
			u.mu.Unlock()
		}
	}
}

func Client(conn net.Conn, u *Users, h *History) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	if !u.CheckPull() {
		fmt.Fprintln(conn, "overflow")
		return
	}

	var username string

	conn.Write([]byte(u.WelcomePig))
	conn.Write([]byte(whatYourName))
	if scanner.Scan() {
		username = scanner.Text()
	} else {
		return
	}

	if err := u.Add(conn, username); err != nil {
		conn.Write([]byte(err.Error()))
		return
	}
	if err := u.glist(conn, username); err != nil {
		conn.Write([]byte(err.Error()))
		return
	}
	defer u.Del(username)

	status <- Message{
		Author: username,
		Msg:    joinChat,
		time:   time.Now(),
	}

	conn.Write([]byte(h.Get()))

	msg := Message{
		Author: username,
		time:   time.Now(),
		conn:   conn,
	}

	msg.PreScan(conn, username)
	for scanner.Scan() {
		text := strings.Trim(scanner.Text(), " ")
		if !isValidtext(text) {
			fmt.Fprintln(conn, "The empty messages are prohibited")
			fmt.Fprintf(conn, "[%s][%s]:", time.Now().Format("2006-01-02 15:04:05"), username)
			continue
		}
		msg.Msg = scanner.Text()
		msg.time = time.Now()

		mess <- msg
	}

	status <- Message{
		Author: username,
		Msg:    leftChat,
		time:   time.Now(),
	}
}

func isValidtext(text string) bool {
	if text == "" {
		return false
	}
	for _, simbol := range text {
		if simbol < 32 || simbol > 127 {
			return false
		}
	}
	return true
}

func ValidName(username string, connection net.Conn) bool {
	if username == "" || len(username) == 0 {
		fmt.Fprintf(connection, "The username is the necessary condition to enter the chat")
		connection.Close()
		return false
	}
	for _, simbol := range username {
		if simbol < 47 || simbol > 122 {
			fmt.Fprintln(connection, "Incorrect input")
			connection.Close()
			// fmt.Fprintf(connection, "[%s][%s]:", time, username)
			return false
		}
	}
	return true
}
