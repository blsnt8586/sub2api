package service

import "github.com/gin-gonic/gin"

const gatewayStreamHeartbeatBytesKey = "gateway_stream_heartbeat_bytes"

// RecordGatewayStreamHeartbeat tracks transport pings emitted while waiting for
// gateway slots. They must not be mistaken for model output by a later attempt.
func RecordGatewayStreamHeartbeat(c *gin.Context, written int) {
	if c == nil || written <= 0 {
		return
	}
	c.Set(gatewayStreamHeartbeatBytesKey, GatewayStreamHeartbeatBytes(c)+written)
}

// GatewayStreamHeartbeatBytes returns only the transport bytes tracked above.
func GatewayStreamHeartbeatBytes(c *gin.Context) int {
	if c == nil {
		return 0
	}
	value, _ := c.Get(gatewayStreamHeartbeatBytesKey)
	written, _ := value.(int)
	return written
}
