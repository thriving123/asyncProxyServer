package transport

type WebsocketProxyResponse struct {
	Success      bool                `msgpack:"success"`
	ErrorMessage string              `msgpack:"errorMessage"`
	Headers      map[string][]string `msgpack:"headers"`
	Body         []byte              `msgpack:"body"`
	RequestId    string              `msgpack:"requestId"`
	StatusCode   int                 `msgpack:"statusCode"`
	EdgeId       string              `msgpack:"edgeId"`
}
