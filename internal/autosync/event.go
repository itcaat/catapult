package autosync

import (
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Op        fsnotify.Op
	Timestamp time.Time
}

// EventType represents the type of file system event
type EventType int

const (
	EventCreate EventType = iota
	EventWrite
	EventRemove
	EventRename
	EventChmod
)

// String returns the string representation of the event type
func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "create"
	case EventWrite:
		return "write"
	case EventRemove:
		return "remove"
	case EventRename:
		return "rename"
	case EventChmod:
		return "chmod"
	default:
		return "unknown"
	}
}

// FromFsnotifyOp converts fsnotify.Op to EventType
func FromFsnotifyOp(op fsnotify.Op) EventType {
	switch {
	case op&fsnotify.Create != 0:
		return EventCreate
	case op&fsnotify.Write != 0:
		return EventWrite
	case op&fsnotify.Remove != 0:
		return EventRemove
	case op&fsnotify.Rename != 0:
		return EventRename
	case op&fsnotify.Chmod != 0:
		return EventChmod
	default:
		return EventWrite // Default to write for safety
	}
}
