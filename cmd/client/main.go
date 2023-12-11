package main

import (
	"asyncProxy/config"
	"asyncProxy/ws/client"
	"fmt"
	"github.com/lxzan/gws"
	"log"
	"time"
)

func main() {
	conf := config.NewConfig("./config.yml")
	var scheme string
	if conf.Client.ServerSecure {
		scheme = "wss"
	} else {
		scheme = "ws"
	}
	addr := fmt.Sprintf("%v://%s:%d/connect", scheme, conf.Client.ServerHost, conf.Client.ServerPort)
	for {
		onCloseSignal := make(chan bool)
		handler := client.WebsocketHandler{
			OnCloseSignal: onCloseSignal,
		}

		wsConnect, response, e := gws.NewClient(&handler, &gws.ClientOption{
			ReadAsyncEnabled: true,
			CompressEnabled:  true,
			Recovery:         gws.Recovery,
			Addr:             addr,
			RequestHeader: map[string][]string{
				"Authorization": {conf.Client.ServerAuthorization},
			},
		})
		if e != nil {
			log.Println("connect error:", e)
			log.Println("server response code:", response.StatusCode)
			log.Println("will retry in 10 secs")
			time.Sleep(10 * time.Second)
			continue
		}
		go func() {
			log.Println("read loop start")
			wsConnect.ReadLoop()
			log.Println("read loop is terminated")
		}()
		e = wsConnect.WritePing([]byte(time.Now().Format(time.RFC822Z)))
		if e != nil {
			log.Println("ping error:", e)
		}
		<-onCloseSignal
		log.Println("connection lost!")
		log.Println("will retry in 10 secs")
		time.Sleep(10 * time.Second)
	}

}
