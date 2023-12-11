package httpProxy

import (
	"asyncProxy/proxy/common"
	"asyncProxy/util"
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type HttpProxyClient struct {
	Host string
	Port uint16
}

// Run 实现proxy的Run方法
func (p *HttpProxyClient) Run() {
	p.Listen()
}

// NewProxy 实现proxy的NewProxy方法
func NewProxy(host string, port uint16) *HttpProxyClient {
	return &HttpProxyClient{
		Host: host,
		Port: port,
	}
}

// Listen 监听客户端请求
func (p *HttpProxyClient) Listen() {
	cert, key, e := common.GenerateFakeCert()
	util.OkOrPanic(e)

	tlsCert, e := tls.X509KeyPair(cert, key)
	util.OkOrPanic(e)
	handler := httpProxyHandler{Cert: tlsCert}

	listener, e := net.Listen("tcp", fmt.Sprintf("%s:%d", p.Host, p.Port))
	util.OkOrPanic(e)

	log.Println("start listening tcp...")
	for {
		conn, err := listener.Accept()
		util.OkOrPanic(err)
		go func() {
			timer := time.After(1 * time.Minute)

			done := make(chan bool)
			go func() {
				handler.ProcessTcpConnection(conn)
				done <- true
			}()
			select {
			case <-done:
			case <-timer:
				_ = conn.Close()
				log.Println("request process timeout")
			}
		}()
	}
}

type httpProxyHandler struct {
	Cert tls.Certificate
}

func (h httpProxyHandler) ProcessTcpConnection(conn net.Conn) {
	defer conn.Close()
	connReader := bufio.NewReader(conn)
	request, e := http.ReadRequest(bufio.NewReader(connReader))
	if e != nil {
		log.Println("tcp connection is not a valid http request, aborting...")
		return
	}

	if request.Method == http.MethodConnect {
		_, e := fmt.Fprint(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		if e != nil {
			log.Println("write error:", e)
			return
		}

		tlsConn := tls.Server(conn, &tls.Config{
			Certificates: []tls.Certificate{h.Cert},
			NextProtos: []string{
				"h2",
				"http/1.1",
			},
		})

		if e := tlsConn.Handshake(); e != nil {
			log.Println("handshake error:", e)
			return
		}
		if tlsConn.ConnectionState().NegotiatedProtocol == "h2" {
			common.ProcessHttp2ProxyRequest(tlsConn, request.URL, "")
		} else {
			common.ProcessHttp11ProxyRequest(tlsConn, request, true, "")
		}
	} else {
		common.ProcessHttp11ProxyRequest(conn, request, false, "")
	}
}
