package errcode

const (
	ErrorNoEdge = 1000 + iota
	ErrorEdgeSendMessageFailed
	ErrorSerializeOrDeserializeFailed
	ErrorNoEdgeIdDefined
	ErrorInvalidEdgeId
	ErrorInvalidResponseMessageType
)
