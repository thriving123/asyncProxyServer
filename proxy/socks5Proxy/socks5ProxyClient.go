package socks5Proxy

import (
	"asyncProxy/errors"
	"asyncProxy/proxy/common"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/things-go/go-socks5"
	"github.com/things-go/go-socks5/statute"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Socks5Proxy struct {
	host string
	port uint16
}

func NewProxy(host string, port uint16) *Socks5Proxy {
	return &Socks5Proxy{
		host: host,
		port: port,
	}
}

func (receiver Socks5Proxy) ListenAndServe() {

	fakeCert, fakeKey, e := common.GenerateFakeCert()
	if e != nil {
		log.Fatalln("error while generate fake cert:", e)
	}

	tlsCert, e := tls.X509KeyPair(fakeCert, fakeKey)
	if e != nil {
		log.Fatalln("error while loading fake cert:", e)
	}
	h := &socks5Handler{
		cert: tlsCert,
	}
	serv := socks5.NewServer(socks5.WithConnectHandle(h.ConnectHandle))

	log.Println("start to listen socks5")
	e = serv.ListenAndServe("tcp", fmt.Sprintf("%s:%d", receiver.host, receiver.port))
	if e != nil {
		log.Fatalln("error while listen to socks5:", e)
	}
}

type socks5Handler struct {
	cert tls.Certificate
}

type socks5NetConn struct {
	Socks5Request *socks5.Request
	Writer        io.Writer
	deadLine      *time.Time
	writeDeadLine *time.Time
	readDeadLine  *time.Time
}

func (s socks5NetConn) Read(b []byte) (n int, err error) {
	var timer time.Duration
	if s.readDeadLine != nil {
		timer = time.Since(*s.readDeadLine)
	} else if s.deadLine != nil {
		timer = time.Since(*s.deadLine)
	}
	if timer < 0 {
		return 0, io.ErrNoProgress
	} else if timer != 0 {
		return readTimeout(s.Socks5Request.Reader, b, timer)
	}

	return s.Socks5Request.Reader.Read(b)
}

func (s socks5NetConn) Write(b []byte) (n int, err error) {
	var timer time.Duration
	if s.writeDeadLine != nil {
		timer = time.Since(*s.writeDeadLine)
	} else if s.deadLine != nil {
		timer = time.Since(*s.deadLine)
	}
	if timer < 0 {
		return 0, io.ErrNoProgress
	} else if timer != 0 {
		return writeTimeout(s.Writer, b, timer)
	}
	return s.Writer.Write(b)
}

func (s socks5NetConn) Close() error {
	return socks5.SendReply(s.Writer, statute.RepServerFailure, s.Socks5Request.LocalAddr)
}

func (s socks5NetConn) LocalAddr() net.Addr {
	return s.Socks5Request.LocalAddr
}

func (s socks5NetConn) RemoteAddr() net.Addr {
	return s.Socks5Request.RemoteAddr
}

func (s socks5NetConn) SetDeadline(t time.Time) error {
	s.deadLine = &t
	return nil
}

func (s socks5NetConn) SetReadDeadline(t time.Time) error {
	s.readDeadLine = &t
	return nil
}

func (s socks5NetConn) SetWriteDeadline(t time.Time) error {
	s.writeDeadLine = &t
	return nil
}

type readWriteResult struct {
	N   int
	Err error
}

func readTimeout(reader io.Reader, b []byte, timeout time.Duration) (n int, err error) {
	timeoutTimer := time.After(timeout)
	readResult := make(chan readWriteResult)
	go func() {
		result, err := reader.Read(b)
		readResult <- readWriteResult{
			N:   result,
			Err: err,
		}
	}()

	select {
	case rs := <-readResult:
		return rs.N, rs.Err
	case <-timeoutTimer:
		return 0, io.ErrNoProgress
	}
}

func writeTimeout(reader io.Writer, b []byte, timeout time.Duration) (n int, err error) {
	timeoutTimer := time.After(timeout)
	readResult := make(chan readWriteResult)
	go func() {
		result, err := reader.Write(b)
		readResult <- readWriteResult{
			N:   result,
			Err: err,
		}
	}()

	select {
	case rs := <-readResult:
		return rs.N, rs.Err
	case <-timeoutTimer:
		return 0, io.ErrNoProgress
	}
}

func (h socks5Handler) ConnectHandle(_ context.Context, writer io.Writer, request *socks5.Request) error {
	processFunc := func(writer io.Writer, request *socks5.Request) error {
		netConn := socks5NetConn{
			Socks5Request: request,
			Writer:        writer,
			deadLine:      nil,
			writeDeadLine: nil,
			readDeadLine:  nil,
		}

		e := socks5.SendReply(writer, statute.RepSuccess, request.LocalAddr)
		if e != nil {
			log.Println("send success failed:", e)
		}
		// 读出首字节
		firstByte := make([]byte, 1)

		_, e = io.ReadFull(request.Reader, firstByte)
		if e != nil {
			log.Println("read first byte error:", e)
			return e
		}

		request.Reader = io.MultiReader(bytes.NewReader(firstByte), request.Reader)

		if isHttps(firstByte[0]) {
			tlsConn := tls.Server(netConn, &tls.Config{
				Certificates: []tls.Certificate{h.cert},
				NextProtos: []string{
					"h2",
					"http/1.1",
				}})
			if e := tlsConn.Handshake(); e != nil {
				log.Println("handshake error:", e)
			}

			if tlsConn.ConnectionState().NegotiatedProtocol == "h2" {
				originUrl := &url.URL{}
				originUrl.Scheme = "https"
				originUrl.Host = tlsConn.ConnectionState().ServerName
				if originUrl.Host == "" {
					originUrl.Host = fmt.Sprintf("%v:%v", request.DstAddr.IP, request.DstAddr.Port)
				}
				common.ProcessHttp2ProxyRequest(tlsConn, originUrl, strconv.Itoa(request.DstAddr.Port))
			} else {
				req, e := http.ReadRequest(bufio.NewReader(tlsConn))
				if e != nil {
					log.Println("malformed http request:", e)
					return e
				}
				common.ProcessHttp11ProxyRequest(tlsConn, req, true, strconv.Itoa(request.DstAddr.Port))
			}

		} else {
			req, e := http.ReadRequest(bufio.NewReader(netConn))
			if e != nil {
				log.Println("malformed http request:", e)
				return e
			}
			common.ProcessHttp11ProxyRequest(netConn, req, false, strconv.Itoa(request.DstAddr.Port))
		}
		return nil
	}

	timer := time.After(1 * time.Minute)
	completeChan := make(chan error)
	go func() {
		e := processFunc(writer, request)
		completeChan <- e
		close(completeChan)
	}()
	select {
	case e := <-completeChan:
		return e
	case <-timer:
		return errors.NewBusinessError(500, "请求处理超时")
	}
}

func isHttps(b byte) bool {
	return b == 22
}
