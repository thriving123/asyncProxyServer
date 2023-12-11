package common

import (
	"asyncProxy/errors"
	"asyncProxy/util"
	"asyncProxy/ws"
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	goerrors "errors"
	"fmt"
	"golang.org/x/net/http2"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var fakeCertKey []byte
var fakeCertCert []byte
var certMux = sync.Mutex{}

func GenerateFakeCert() (cert []byte, key []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			cert = nil
			key = nil
			if r, ok := r.(error); ok {
				err = r
			} else {
				err = goerrors.New("unknown error")
			}
		}
	}()
	certMux.Lock()
	defer certMux.Unlock()
	if fakeCertCert != nil && fakeCertKey != nil {
		return fakeCertCert, fakeCertKey, nil
	}

	log.Println("start generate cert")
	pk, err := rsa.GenerateKey(rand.Reader, 4096)
	util.OkOrPanic(err)
	tpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(2, 0, 0),
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	c, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &pk.PublicKey, pk)
	util.OkOrPanic(err)

	certBuf := bytes.NewBuffer(nil)
	e := pem.Encode(certBuf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c,
	})
	util.OkOrPanic(e)

	keyBuf := bytes.NewBuffer(nil)
	e = pem.Encode(keyBuf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	})
	util.OkOrPanic(e)

	log.Println("generate cert complete")
	fakeCertCert = certBuf.Bytes()
	fakeCertKey = keyBuf.Bytes()
	return fakeCertCert, fakeCertKey, nil
}

type ProxyHttp2Handler struct {
	OriginUrl  *url.URL
	OriginPort string
}

func (h ProxyHttp2Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Host == "" && h.OriginUrl.Host != "" {
		request.URL.Host = h.OriginUrl.Host
	}
	response, e := processHttp11Request(request, h.OriginPort)
	if e != nil {
		response = convertErrorToResponse(request, e)
	}
	for key, value := range response.Header {
		for _, val := range value {
			writer.Header().Add(key, val)
		}
	}
	writer.WriteHeader(response.StatusCode)
	content, e := readRequestBody(response.Body)
	if e != nil {
		log.Println("read response body error:", e)
	}
	_, e = writer.Write(content)
	if e != nil {
		log.Println("http2 write error:", e)
	}
}

func readRequestBody(reader io.Reader) (content []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			content = nil
			if r, ok := r.(error); ok {
				err = r
			} else {
				err = goerrors.New("unknown error")
			}
		}
	}()

	c, e := io.ReadAll(reader)
	util.OkOrPanic(e)
	content = c
	return
}

func convertErrorToResponse(request *http.Request, e error) *http.Response {
	response := &http.Response{
		Header:     map[string][]string{},
		Request:    request,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
	}
	response.Header.Add("x-proxy-error", "1")
	switch ev := e.(type) {
	case *errors.BusinessError:
		code := 0
		if ev.Code > 599 {
			code = 500
		} else {
			code = ev.Code
		}
		response.StatusCode = code
		response.Status = "RequestFailed"
		response.Body = io.NopCloser(bytes.NewReader([]byte(ev.Message)))
	default:
		response.StatusCode = 500
		response.Status = "InternalServerError"
		response.Body = io.NopCloser(bytes.NewReader([]byte(fmt.Sprintln("内部错误:", ev.Error()))))
	}
	return response
}

func processHttp11Request(request *http.Request, port string) (response *http.Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			response = nil
			if r, ok := r.(error); ok {
				err = r
			} else {
				err = goerrors.New("unknown error")
			}
		}
	}()
	actualUrl := ""

	if request.URL.IsAbs() {
		actualUrl = request.URL.String()
	} else {
		if request.URL.Scheme == "" {
			if request.TLS != nil && request.TLS.HandshakeComplete {
				actualUrl = "https://"
			} else {
				actualUrl = "http://"
			}
		} else {
			actualUrl = request.URL.Scheme
			actualUrl = actualUrl + "://"
		}

		if request.URL.Hostname() == "" {
			host := request.Host
			if host == "" {
				errors.NewBusinessError(400, "No host specific").Panic()
			}
			actualUrl = actualUrl + host
			if port != "" && !strings.Contains(host, ":") {
				actualUrl = actualUrl + ":" + port
			}
		} else {
			actualUrl = actualUrl + request.URL.Hostname()
			if port != "" {
				actualUrl = actualUrl + ":" + port
			} else if request.URL.Port() != "" {
				actualUrl = actualUrl + ":" + request.URL.Port()
			}
		}

		if strings.HasPrefix(request.URL.Path, "/") {
			actualUrl = actualUrl + request.URL.Path
		} else {
			actualUrl = actualUrl + "/" + request.URL.Path
		}
		actualUrl = actualUrl + "?" + request.URL.RawQuery
	}

	reqBody, e := readRequestBody(request.Body)
	util.OkOrPanic(e)

	wsResponse, err := ws.SendRequestAndWait(request.Method, actualUrl,
		request.Header, reqBody, 30*time.Second)
	util.OkOrPanic(err)

	if !wsResponse.Success {
		errors.NewBusinessError(503, fmt.Sprint("边缘节点返回错误信息:", wsResponse.ErrorMessage)).Panic()
	}

	resp := &http.Response{
		Header:     wsResponse.Headers,
		StatusCode: wsResponse.StatusCode,
		Request:    request,
		Body:       io.NopCloser(bytes.NewReader(wsResponse.Body)),
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
	}

	return resp, nil
}

func ProcessHttp2ProxyRequest(netConn net.Conn, originUrl *url.URL, originPort string) {
	h := ProxyHttp2Handler{
		OriginUrl:  originUrl,
		OriginPort: originPort,
	}
	h2s := http2.Server{}
	h2s.ServeConn(netConn, &http2.ServeConnOpts{Handler: h, SawClientPreface: false, Settings: []byte{}})
}

func ProcessHttp11ProxyRequest(netConn net.Conn, request *http.Request, isConnect bool, port string) {
	if isConnect {
		h11Req, e := http.ReadRequest(bufio.NewReader(netConn))
		if e != nil {
			log.Println("read tls conn error:", e)
			response := convertErrorToResponse(request, e)
			e := response.Write(netConn)
			if e != nil {
				log.Println("write to tls conn error:", e)
			}
			return
		}
		if h11Req.URL.Host == "" && request.URL.Host != "" {
			h11Req.URL.Host = request.URL.Host
		}
		request = h11Req
	}

	response, e := processHttp11Request(request, port)
	if e != nil {
		errorResponse := convertErrorToResponse(request, e)
		e := errorResponse.Write(netConn)
		if e != nil {
			log.Println("write error response to net.Conn error:", e)
		}
		return
	}
	e = response.Write(netConn)
	if e != nil {
		log.Println("write to net.Conn error:", e)
	}
}
