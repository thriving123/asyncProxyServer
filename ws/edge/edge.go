package edge

import (
	"asyncProxy/constant"
	"asyncProxy/constant/errcode"
	"asyncProxy/errors"
	"asyncProxy/ws/transport"
	"cmp"
	"github.com/lxzan/gws"
	"github.com/oklog/ulid/v2"
	"github.com/vmihailenco/msgpack/v5"
	"slices"
	"sync"
	"time"
)

type OnResponseCompleteCallback func(response *transport.WebsocketProxyResponse)
type OnResponseTimeoutCallback func(requestId string)

type requestCallback struct {
	Complete     OnResponseCompleteCallback
	CompleteChan chan int
}

type Edge struct {
	Conn       *gws.Conn
	EdgeId     string
	LastUsedAt time.Time
}

type EdgeSet struct {
	edges     []*Edge
	callbacks sync.Map

	sync.RWMutex // for safely operate edges
}

func NewEdgeSet() *EdgeSet {
	return &EdgeSet{
		edges:     []*Edge{},
		callbacks: sync.Map{},
		RWMutex:   sync.RWMutex{},
	}
}

func (s *EdgeSet) Len() int {
	s.RWMutex.RLock()
	defer s.RWMutex.RUnlock()
	return len(s.edges)
}

func (s *EdgeSet) Add(conn *gws.Conn) string {
	edgeId := ulid.Make().String()
	conn.Session().Store(constant.ConnSessionEdgeId, edgeId)
	edge := &Edge{
		Conn:       conn,
		EdgeId:     edgeId,
		LastUsedAt: time.Unix(0, 0),
	}
	s.RWMutex.Lock()
	defer s.RWMutex.Unlock()

	s.edges = append(s.edges, edge)
	return edgeId
}

func (s *EdgeSet) Remove(edgeId string) {
	s.RWMutex.Lock()
	defer s.RWMutex.Unlock()
	newSlice := slices.DeleteFunc(s.edges, func(edge *Edge) bool {
		return edge.EdgeId == edgeId
	})
	s.edges = newSlice
}

func (s *EdgeSet) RemoveByConnection(conn *gws.Conn) (err error) {
	var edgeId string
	if anyEdgeId, exists := conn.Session().Load(constant.ConnSessionEdgeId); exists {
		if eid, ok := anyEdgeId.(string); ok {
			edgeId = eid
		} else {
			return errors.NewBusinessError(errcode.ErrorInvalidEdgeId, "边缘节点ID类型错误")
		}
	} else {
		return errors.NewBusinessError(errcode.ErrorNoEdgeIdDefined, "边缘节点ID未定义")
	}

	s.Remove(edgeId)

	return nil
}

func (s *EdgeSet) DispatchRequest(method, url string, headers map[string][]string, body []byte, timeout time.Duration,
	completeCallback OnResponseCompleteCallback) (reqId string, err error) {
	if len(s.edges) == 0 {
		return "", errors.NewBusinessError(errcode.ErrorNoEdge, "边缘节点为空")
	}
	s.RWMutex.RLock()
	defer s.RWMutex.RUnlock()
	sortAndReturnEdge := func() *Edge {
		copyEdges := slices.Clone(s.edges)
		slices.SortFunc(copyEdges, func(a, b *Edge) int {
			return cmp.Compare(a.LastUsedAt.Unix(), b.LastUsedAt.Unix())
		})
		firstEdge := s.edges[0]
		firstEdge.LastUsedAt = time.Now()
		return firstEdge
	}

	firstEdge := sortAndReturnEdge()

	requestId := ulid.Make().String()
	request := transport.WebsocketProxyRequest{
		FullUrl:   url,
		Headers:   headers,
		Body:      body,
		RequestId: requestId,
		Method:    method,
		Timeout:   timeout.Seconds(),
		EdgeId:    firstEdge.EdgeId,
	}

	b, e := msgpack.Marshal(&request)
	if e != nil {
		return "", errors.NewBusinessError(errcode.ErrorSerializeOrDeserializeFailed, "HTTP请求序列化失败").WithInnerError(e)
	}
	e = firstEdge.Conn.WriteMessage(gws.OpcodeBinary, b)

	if e != nil {
		return "", errors.NewBusinessError(errcode.ErrorEdgeSendMessageFailed, "发送请求到边缘节点失败").WithInnerError(e)
	}
	completeChan := make(chan int)

	callbackStruct := requestCallback{
		Complete:     completeCallback,
		CompleteChan: completeChan,
	}

	s.callbacks.Store(requestId, &callbackStruct)

	go func() {
		timeoutTimer := time.After(timeout)
		select {
		case <-timeoutTimer:
			timeoutResponse := transport.WebsocketProxyResponse{
				Success:      false,
				ErrorMessage: "Request Timeout",
				Headers:      nil,
				Body:         nil,
				RequestId:    requestId,
				StatusCode:   -1,
				EdgeId:       firstEdge.EdgeId,
			}
			completeCallback(&timeoutResponse)
			s.callbacks.Delete(requestId)
		case <-completeChan:
			s.callbacks.Delete(requestId)
		}
	}()
	return requestId, nil
}

func (s *EdgeSet) DispatchRequestAndWait(method, url string, headers map[string][]string, body []byte, timeout time.Duration) (response *transport.WebsocketProxyResponse, err error) {
	messageChan := make(chan *transport.WebsocketProxyResponse)

	completeCallback := func(response *transport.WebsocketProxyResponse) {
		messageChan <- response
	}

	_, e := s.DispatchRequest(method, url, headers, body, timeout, completeCallback)
	if e != nil {
		return nil, e
	}

	result := <-messageChan
	return result, nil
}

func (s *EdgeSet) OnResponse(conn *gws.Conn, message *gws.Message) (response *transport.WebsocketProxyResponse, err error) {
	defer message.Close()
	if message.Opcode != gws.OpcodeBinary {
		return nil, errors.NewBusinessError(errcode.ErrorInvalidResponseMessageType, "错误的消息类型")
	}
	var edgeId string
	if anyEdgeId, exists := conn.Session().Load(constant.ConnSessionEdgeId); exists {
		if eid, ok := anyEdgeId.(string); ok {
			edgeId = eid
		} else {
			return nil, errors.NewBusinessError(errcode.ErrorInvalidEdgeId, "边缘节点ID类型错误")
		}
	} else {
		return nil, errors.NewBusinessError(errcode.ErrorNoEdgeIdDefined, "边缘节点ID未定义")
	}
	messageBytes := message.Bytes()
	var wsResponse transport.WebsocketProxyResponse
	e := msgpack.Unmarshal(messageBytes, &wsResponse)
	if e != nil {
		return nil, errors.NewBusinessError(errcode.ErrorSerializeOrDeserializeFailed, "边缘节点响应解码失败").WithInnerError(e)
	}

	wsResponse.EdgeId = edgeId

	callback, _ := s.callbacks.Load(wsResponse.RequestId)
	if callback != nil {
		if c, ok := callback.(*requestCallback); ok {
			c.Complete(&wsResponse)
			c.CompleteChan <- 0
			close(c.CompleteChan)
		}
		s.callbacks.Delete(wsResponse.RequestId)
	}

	return &wsResponse, nil
}

func (s *EdgeSet) Data() []*Edge {
	return s.edges
}
