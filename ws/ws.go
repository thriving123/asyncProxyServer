package ws

import (
	"asyncProxy/ws/edge"
	"asyncProxy/ws/transport"
	"fmt"
	"github.com/lxzan/gws"
	"log"
	"net/http"
	"time"
)

var EdgeSet = edge.NewEdgeSet()

type Handler struct {
}

func (c *Handler) OnOpen(socket *gws.Conn) {
	edgeId := EdgeSet.Add(socket)
	log.Println("连接建立, 分配的节点ID为:", edgeId)
	log.Println("当前在线节点数为:", EdgeSet.Len())
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {
	log.Println("错误原因：" + err.Error())
	log.Println("连接关闭！")
	e := EdgeSet.RemoveByConnection(socket)
	if e != nil {
		log.Println("移除节点失败:", e)
	}
	log.Println("当前在线节点数为:", EdgeSet.Len())
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	// 返回pong字符串
	if err := socket.WritePong(payload); err != nil {
		log.Println("发送Pong消息失败！")
	}
}

func (c *Handler) OnPong(_ *gws.Conn, _ []byte) {}

func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	_, e := EdgeSet.OnResponse(socket, message)
	if e != nil {
		log.Println("websocket消息处理失败:", e)
	}
}

func Start(host string, port uint16, authorization string) {
	var handler *Handler = &Handler{}
	upgrader := gws.NewUpgrader(handler, &gws.ServerOption{
		ReadAsyncEnabled: true,
		CompressEnabled:  true,
		Recovery:         gws.Recovery,
	})
	http.HandleFunc("/connect", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != authorization {
			writer.Header().Set("WWW-Authenticate", "Invalid request authorization")
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		socket, err := upgrader.Upgrade(writer, request)
		if err != nil {
			log.Println("ws 连接异常！")
			_, _ = writer.Write([]byte("websocket connect error"))
			return
		}
		go func() {
			socket.ReadLoop()
		}()
	})
	log.Println("start listen ws")
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil); err != nil {
		log.Panicln("ws 启动异常！")
	}
	log.Println("ws 启动成功！")

}

// SendRequest 发送请求
func SendRequest(method, url string, headers map[string][]string, body []byte, timeout time.Duration, callback edge.OnResponseCompleteCallback) error {
	_, e := EdgeSet.DispatchRequest(method, url, headers, body, timeout, callback)
	return e
}

// SendRequestAndWait 发送请求然后等待请求结果
func SendRequestAndWait(method, url string, headers map[string][]string, body []byte, timeout time.Duration) (*transport.WebsocketProxyResponse, error) {
	response, e := EdgeSet.DispatchRequestAndWait(method, url, headers, body, timeout)
	return response, e
}
