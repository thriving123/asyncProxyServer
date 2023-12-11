package transport

type WebsocketProxyRequest struct {
	FullUrl   string              `msgpack:"fullUrl"`
	Headers   map[string][]string `msgpack:"headers"`
	Body      []byte              `msgpack:"body"`
	Method    string              `msgpack:"method"`
	RequestId string              `msgpack:"requestId"`
	Timeout   float64             `msgpack:"timeout"`
	EdgeId    string              `msgpack:"edgeId"`
}
