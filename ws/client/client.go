package client

import (
	"asyncProxy/ws/transport"
	"github.com/lxzan/gws"
	"github.com/vmihailenco/msgpack/v5"
	"io"
	"log"
	"time"
)

type WebsocketHandler struct {
	OnCloseSignal chan bool
}

func (w *WebsocketHandler) OnOpen(_ *gws.Conn) {
	log.Println("websocket connected!")
}

func (w *WebsocketHandler) OnClose(_ *gws.Conn, _ error) {
	log.Println("websocket connection lost!")
	w.OnCloseSignal <- true
}

func (w *WebsocketHandler) OnPing(_ *gws.Conn, _ []byte) {

}

func (w *WebsocketHandler) OnPong(socket *gws.Conn, _ []byte) {
	time.Sleep(10 * time.Second)
	e := socket.WritePing([]byte(time.Now().Format(time.RFC822Z)))
	if e != nil {
		log.Println("ping error:", e)
	}
}

func (w *WebsocketHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	if message.Opcode != gws.OpcodeBinary {
		log.Println("invalid opcode:", message.Opcode)
		return
	}

	var wsRequest transport.WebsocketProxyRequest

	e := msgpack.Unmarshal(message.Bytes(), &wsRequest)
	if e != nil {
		log.Println("msgpack unmarshal error:", e)
		return
	}
	httpRequest := httpClient.R()
	for key, value := range wsRequest.Headers {
		for _, val := range value {
			httpRequest.SetHeader(key, val)
		}
	}

	httpRequest.SetBodyBytes(wsRequest.Body)

	response, e := httpRequest.Send(wsRequest.Method, wsRequest.FullUrl)

	wsResponse := &transport.WebsocketProxyResponse{}

	if e != nil {
		wsResponse.Success = false
		wsResponse.ErrorMessage = e.Error()
	} else {
		wsResponse.Success = true
		wsResponse.Headers = response.Header
		wsResponse.RequestId = wsRequest.RequestId
		wsResponse.StatusCode = response.StatusCode
		wsResponse.EdgeId = wsRequest.EdgeId

		defer response.Body.Close()
		bodyBytes, e := io.ReadAll(response.Body)
		if e != nil {
			log.Println("body read error:", e)
			wsResponse.Success = false
			wsResponse.ErrorMessage = e.Error()
			return
		} else {
			wsResponse.Body = bodyBytes
		}
	}

	wsBytes, e := msgpack.Marshal(&wsResponse)
	if e != nil {
		log.Println("serialize response error:", e)
		return
	}
	e = socket.WriteMessage(gws.OpcodeBinary, wsBytes)
	if e != nil {
		log.Println("send response error:", e)
	}
}
