package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type client struct {
	struconn net.Conn
	struname string
}

type message struct {
	userName string
	text     string
	address  string
	time     string
}

var (
	messagechan = make(chan message)
	join        = make(chan message)
	left        = make(chan message)
	Ourclients  = make(map[string]client)
	errorChan   = make(chan string)
)

var HelloMessage = []string{
	"Welcome to TCP-Chat!\n",
	"         _nnnn_\n",
	"        dGGGGMMb\n",
	"       @p~qp~~qMb\n",
	"       M|@||@) M|\n",
	"       @,----.JM|\n",
	"      JS^\\__/  qKL\n",
	"     dZP        qKRb\n",
	"    dZP          qKKb\n",
	"   fZP            SMMb\n",
	"   HZM            MMMM\n",
	"   FqM            MMMM\n",
	" __| \".        |\\dS\"qML\n",
	" |    `.       | `' \\Zq\n",
	"_)      \\.___.,|     .'\n",
	"\\____   )MMMMMP|   .'\n",
	"     `-'       `--'\n",
	"[ENTER YOUR NAME]: ",
}

func main() {
	port := "8081"
	if len(os.Args) > 2 {
		fmt.Println("[USAGE]: ./TCPChat $port")
		return
	}

	portFunc(&port)
	fmt.Println("Listening on the port :", port)
	listen, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}
	if _, err := os.Create("chat.txt"); err != nil {
		log.Printf("Failed to create: %v", err)
		return
	}
	if err := os.Truncate("chat.txt", 0); err != nil {
		log.Printf("Failed to truncate: %v", err)
		return
	}
	defer listen.Close()
	mu := &sync.Mutex{}
	go broadCast(mu)
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println(err.Error(), conn.RemoteAddr().String)
			continue
		}
		go newConnection(conn, mu)
	}
}

func portFunc(port *string) {
	if len(os.Args) == 1 {
		return
	}

	if num := Atoi(os.Args[1]); num < 1024 || num > 65535 {
		return
	}
	*port = os.Args[1]
}

func newConnection(conn net.Conn, mu *sync.Mutex) {
	NewUser, errBool := newUser(conn, mu)

	if !errBool {
		conn.Close()
		return
	}

	mu.Lock()
	if len(Ourclients) > 10 {
		fmt.Fprintln(conn, "Chatroom is full.please, come back later")
		conn.Close()
		mu.Unlock()
		return
	}
	mu.Unlock()

	address := conn.RemoteAddr().String()

	Ourclients[address] = NewUser

	join <- newMessage("joined.", Ourclients[address])

	mu.Lock()
	messagehistory(conn)

	mu.Unlock()

	input := bufio.NewScanner(conn)
	for input.Scan() {
		msg := strings.Trim(input.Text(), " ")
		if !checkMessage(msg) {
			fmt.Fprint(NewUser.struconn, time.Now().Format("01-02-2006 15:04:05")+"["+NewUser.struname+"]"+":")
			continue
		}

		msgStru := newMessage(msg, NewUser)
		appendmessagehistory(msgStru.time + "[" + msgStru.userName + "]" + ":" + msgStru.text + "\n")
		messagechan <- msgStru
	}
	mu.Lock()
	left <- newMessage(" has left.", Ourclients[address])
	fmt.Println("quitted", Ourclients[address])
	mu.Unlock()

	mu.Lock()
	delete(Ourclients, address)
	mu.Unlock()

	conn.Close()
	return
}

func newUser(conn net.Conn, mu *sync.Mutex) (client, bool) {
	for _, x := range HelloMessage {
		fmt.Fprint(conn, x)
	}
	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	name := scanner.Text()
	var NewUser client
	if !checkName(name, conn, mu) {
		return NewUser, false
	}
	NewUser = client{struconn: conn, struname: name}
	return NewUser, true
}

func newMessage(text string, user client) message {
	newTime := "[" + time.Now().Format("01-02-2006 15:04:05") + "]"
	newAddr := user.struconn.RemoteAddr().String()
	return message{
		time:     newTime,
		userName: user.struname,
		text:     text,
		address:  newAddr,
	}
}

func checkName(name string, conn net.Conn, mu *sync.Mutex) bool {
	for _, ch := range name[:len(name)-1] {
		if ch >= 0 && ch <= 31 {
			fmt.Fprint(conn, "Wrong name")
			return false
		}
	}
	if len(name) == 0 {
		return false
	}
	mu.Lock()
	for _, client := range Ourclients {
		if client.struname == name {
			fmt.Fprint(conn, "Username already exist")
			mu.Unlock()
			return false
		}
	}
	mu.Unlock()
	return true
}

func broadCast(mu *sync.Mutex) {
	for {
		select {
		case msg := <-messagechan:
			mu.Lock()
			for _, connStru := range Ourclients {
				if connStru.struconn.RemoteAddr().String() != msg.address {
					fmt.Fprint(connStru.struconn, "\n"+msg.time+"["+msg.userName+"]"+":"+msg.text+"\n"+msg.time+"["+connStru.struname+"]"+":")
					continue
				}
				fmt.Fprint(connStru.struconn, msg.time+"["+connStru.struname+"]"+":")
			}

			mu.Unlock()
		case joi := <-join:
			mu.Lock()
			for _, connStru := range Ourclients {
				if connStru.struconn.RemoteAddr().String() != joi.address {
					fmt.Fprintln(connStru.struconn, "\n"+joi.userName+" has joined our chat...")
				}
				fmt.Fprint(connStru.struconn, joi.time+"["+connStru.struname+"]"+":")
			}
			mu.Unlock()
		case lef := <-left:
			mu.Lock()
			for _, connStru := range Ourclients {
				// TODO less messages
				fmt.Fprintln(connStru.struconn, "\n"+lef.userName+" has left our chat...")
				fmt.Fprint(connStru.struconn, lef.time+"["+connStru.struname+"]"+":")
			}
			mu.Unlock()
		case errorMessage := <-errorChan:
			mu.Lock()
			for _, connStru := range Ourclients {
				fmt.Fprintln(connStru.struconn, "\n"+errorMessage)
				connStru.struconn.Close()
			}
			mu.Unlock()
		}
	}
}

func checkMessage(text string) bool {
	if text == "" {
		return false
	}

	for _, char := range text {
		if char < 32 || char > 126 {
			return false
		}
	}
	return true
}

func messagehistory(conn net.Conn) {
	file, err := ioutil.ReadFile("chat.txt")
	if err != nil {
		fmt.Println(err)
		errorChan <- "Internal error, sorry! Bye, bye!"
		os.Exit(0)
	}
	chatCache := string(file)
	if len(chatCache) == 0 {
		return
	}
	fmt.Fprint(conn, chatCache)
}

func appendmessagehistory(str string) {
	f, err := os.OpenFile("chat.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	defer f.Close()

	if err != nil {
		errorChan <- "Internal error, sorry! Bye, bye!"
		log.Fatal(err)
	}
	if _, err := f.Write([]byte(str)); err != nil {
		errorChan <- "Internal error, sorry! Bye, bye!"
		log.Fatal(err)
		os.Exit(0)
	}
	if err := f.Close(); err != nil {
		errorChan <- "Internal error, sorry! Bye, bye!"
		log.Fatal(err)
		os.Exit(0)
	}
}

func Atoi(s string) int {
	if len(s) < 1 {
		return 0
	}
	chars := []rune(s)
	result := 0
	for i := 0; i < len(chars); i++ {
		if chars[i] >= '0' && chars[i] <= '9' {
			result = result*10 + (int(chars[i]) - '0')
		} else {
			return 0
		}
	}
	return result
}
