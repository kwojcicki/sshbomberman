package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"

	"github.com/fatih/color"

	"bufio"

	"golang.org/x/crypto/ssh"

	"bytes"
)

func main() {

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:2022")
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}
	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Fatal("failed to accept incoming connection: ", err)
		}

		// Before use, a handshake must be performed on the incoming
		// net.Conn.
		_, chans, reqs, err := ssh.NewServerConn(nConn, config)
		if err != nil {
			log.Fatal("failed to handshake: ", err)
		}

		// The incoming Request channel must be serviced.
		go ssh.DiscardRequests(reqs)

		// Service the incoming Channel channel.
		for newChannel := range chans {
			// Channels have a type, depending on the application level
			// protocol intended. In the case of a shell, the type is
			// "session" and ServerShell may be used to present a simple
			// terminal interface.
			if newChannel.ChannelType() != "session" {
				newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				log.Fatalf("Could not accept channel: %v", err)
			}

			// Reject all out of band requests accept for the unix defaults, pty-req and
			// shell.
			go func(in <-chan *ssh.Request) {
				for req := range in {
					switch req.Type {
					case "pty-req":
						req.Reply(true, nil)
						continue
					case "shell":
						req.Reply(true, nil)
						continue
					}
					req.Reply(false, nil)
				}
			}(requests)

			go func() {
				defer channel.Close()
				sess := &Session{channel}
				reader := bufio.NewReader(channel)
				for {
					line, _, err := reader.ReadRune()
					if err != nil {
						break
					}
					fmt.Println(line)

					g := createGame()
					stringMap := g.generateMap()
					var b bytes.Buffer
					b.WriteString("\033[H\033[2J\033[?25l")
					b.WriteString(stringMap)
					_, err = io.Copy(sess, &b)
					switch line {
					case 3:
						sess.Write([]byte("\033[?25h"))
						return
					}
					if err != nil {
						break
					}

				}
			}()
		}
	}
}

func createGame() *Game {
	gameTiles := make([][]int, 78)

	for x := range gameTiles {
		gameTiles[x] = make([]int, 44)
		for y := range gameTiles[x] {
			gameTiles[x][y] = 0
		}
	}
	return &Game{
		tiles: gameTiles,
	}
}

type Game struct {
	tiles [][]int
}

type Session struct {
	c ssh.Channel
}

// Characters for rendering
const (
	verticalWall   = '║'
	horizontalWall = '═'
	topLeft        = '╔'
	topRight       = '╗'
	bottomRight    = '╝'
	bottomLeft     = '╚'

	grass        = ' '
	blocker      = '■'
	playerOnBomb = 'Ⓧ'
	player       = 'x'
	bomb         = '⭕'
	bombTwo      = '🞅'
	bomb1        = '◔'
	bomb2        = '◑'
	bomb3        = '◕'
	bomb4        = '⚫'
)

func (g *Game) generateMap() string {
	height := 44
	width := 78

	strWorld := make([][]string, width+2)

	colorFunc := color.New(color.FgRed).SprintFunc()
	for x := range strWorld {
		strWorld[x] = make([]string, height+2)
	}

	strWorld[0][0] = colorFunc(string(topLeft))
	strWorld[width+1][0] = colorFunc(string(topRight))
	strWorld[0][height+1] = colorFunc(string(bottomLeft))
	strWorld[width+1][height+1] = colorFunc(string(bottomRight))
	for x := 1; x < width; x++ {
		strWorld[x][0] = colorFunc(string(horizontalWall))
		strWorld[x][height+1] = colorFunc(string(horizontalWall))
	}

	for y := 1; y <= height; y++ {
		strWorld[0][y] = colorFunc(string(verticalWall))
		strWorld[width+1][y] = colorFunc(string(verticalWall))
	}

	buffer := bytes.NewBuffer(make([]byte, 0, (height+2)*(width+2)))
	for y := 0; y < len(strWorld[0]); y++ {
		for x := 0; x < len(strWorld); x++ {
			buffer.WriteString(strWorld[x][y])
		}

		buffer.WriteString("\r\n")
	}
	return buffer.String()
}

func (s *Session) Read(p []byte) (int, error) {
	return s.c.Read(p)
}

func (s *Session) Write(p []byte) (int, error) {
	return s.c.Write(p)
}
