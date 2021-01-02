package cmdchat

import "time"

const (

	// DefaultMaxMessageSize denotes the default maximum size allowed for transmission (32 MiB)
	DefaultMaxMessageSize = 30 << 20

	// DefaultWriteTimeout denotes the default timeout for a write operation
	DefaultWriteTimeout = 10 * time.Second

	// DefaultKeepAliveInterval denotes the default interval for keepalive pings
	DefaultKeepAliveInterval = 30 * time.Second

	// DefaultKeepAliveDeadline denotes the default deadline for keepalive pings
	DefaultKeepAliveDeadline = 60 * time.Second
)
