package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/turn/v2"
)

var upgrader = websocket.Upgrader{}

var lock sync.Mutex
var clients = make(map[*websocket.Conn]bool)

func signal(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	lock.Lock()
	clients[c] = true
	lock.Unlock()

	defer func() {
		lock.Lock()
		delete(clients, c)
		lock.Unlock()
	}()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("message type: %d, recv: %s", mt, message)

		for k := range clients {
			k.WriteMessage(mt, message)
		}
	}

}

func home(w http.ResponseWriter, r *http.Request) {
	log.Println("handle ", r.URL.Path)

	if r.URL.Path == "/" {
		f, err := ioutil.ReadFile("client/index.html")
		if err != nil {
			log.Println(err)
			return
		}
		w.Header().Add("Content-Type", "text/html")
		w.Write(f)
	} else if r.URL.Path == "/webrtc_promise.js" {
		f, err := ioutil.ReadFile("client/webrtc_promise.js")
		if err != nil {
			log.Println(err)
			return
		}
		w.Header().Add("Content-Type", "application/javascript")
		w.Write(f)
	} else if r.URL.Path == "/webrtc_async.js" {
		f, err := ioutil.ReadFile("client/webrtc_async.js")
		if err != nil {
			log.Println(err)
			return
		}
		w.Header().Add("Content-Type", "application/javascript")
		w.Write(f)
	}

}

func main() {
	go startTurnServer() // 开启turn服务
	http.HandleFunc("/", home)
	http.HandleFunc("/signal", signal)
	// http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", nil)
	http.ListenAndServe(":8443", nil)
}

func startTurnServer() {
	udpListener, err := net.ListenPacket("udp4", ":8444")
	if err != nil {
		log.Printf("Failed to create TURN server listener: %s", err)
		return
	}

	usersMap := map[string][]byte{}
	usersMap["webrtc-demo"] = turn.GenerateAuthKey("webrtc-demo", "webrtc-demo-turn", "123456")

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: "webrtc-demo-turn",
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			log.Printf("username: %s, realm: %s", username, realm)
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP("0.0.0.0"), // 这里应该填公网ip
					Address:      "0.0.0.0",              // But actually be listening on every interface
				},
			},
		},
	})

	if err != nil {
		log.Println(err)
	}

	defer s.Close()

	select {}
}
