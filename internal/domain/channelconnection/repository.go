package channelconnection

import (
	"context"
	"errors"
)

var ErrConflict = errors.New("connection revision conflict")
var ErrNotFound = errors.New("connection not found")
var ErrUnconfigured = errors.New("system channel not configured")
var ErrDisconnected = errors.New("channel connection disconnected")
var ErrWritesDisabled = errors.New("encrypted connection writes are disabled until all readers are upgraded")

type Repository interface {
	Get(context.Context, string) (*Connection, error)
	GetRevision(context.Context, string, int64) (*Connection, error)
	Save(context.Context, *Connection, int64) error
	FindSender(context.Context, string, string, string) ([]*Connection, error)
	FindAccount(context.Context, string, string) ([]*Connection, error)
}
