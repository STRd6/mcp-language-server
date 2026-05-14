package testing

import (
	"context"
	"sync"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
)

// FileEvent represents a file event notification
type FileEvent struct {
	URI  string
	Type protocol.FileChangeType
}

// MockLSPClient implements the watcher.LSPClient interface for testing
type MockLSPClient struct {
	mu             sync.Mutex
	events         []FileEvent
	openedFiles    map[string]bool
	openErrors     map[string]error
	notifyErrors   map[string]error
	changeErrors   map[string]error
	eventsReceived chan struct{}
}

// NewMockLSPClient creates a new mock LSP client for testing
func NewMockLSPClient() *MockLSPClient {
	return &MockLSPClient{
		events:         []FileEvent{},
		openedFiles:    make(map[string]bool),
		openErrors:     make(map[string]error),
		notifyErrors:   make(map[string]error),
		changeErrors:   make(map[string]error),
		eventsReceived: make(chan struct{}, 100), // Buffer to avoid blocking
	}
}

// IsFileOpen checks if a file is already open in the editor
func (m *MockLSPClient) IsFileOpen(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.openedFiles[path]
}

// OpenFile mocks opening a file in the editor
func (m *MockLSPClient) OpenFile(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, ok := m.openErrors[path]; ok {
		return err
	}

	m.openedFiles[path] = true
	return nil
}

// NotifyChange mocks notifying the server of a file change
func (m *MockLSPClient) NotifyChange(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, ok := m.notifyErrors[path]; ok {
		return err
	}

	// Record this as a change event
	m.events = append(m.events, FileEvent{
		URI:  string(protocol.URIFromPath(path)),
		Type: protocol.FileChangeType(protocol.Changed),
	})

	// Signal that an event was received
	select {
	case m.eventsReceived <- struct{}{}:
	default:
		// Channel is full, but we don't want to block
	}

	return nil
}

// DidChangeWatchedFiles mocks sending watched file events to the server
func (m *MockLSPClient) DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, change := range params.Changes {
		uri := string(change.URI)

		if err, ok := m.changeErrors[uri]; ok {
			return err
		}

		// Record the event
		m.events = append(m.events, FileEvent{
			URI:  uri,
			Type: change.Type,
		})
	}

	// Signal that an event was received
	select {
	case m.eventsReceived <- struct{}{}:
	default:
		// Channel is full, but we don't want to block
	}

	return nil
}

// GetEvents returns a copy of all recorded events
func (m *MockLSPClient) GetEvents() []FileEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Make a copy to avoid race conditions
	result := make([]FileEvent, len(m.events))
	copy(result, m.events)
	return result
}

// CountEvents counts events for a specific file and event type
func (m *MockLSPClient) CountEvents(uri string, eventType protocol.FileChangeType) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, evt := range m.events {
		if evt.URI == uri && evt.Type == eventType {
			count++
		}
	}
	return count
}

// ResetEvents clears the recorded events and drains any pending notification
// signals from previous activity (e.g. workspace scan, prior subtests). Without
// the drain, WaitForEvent could return immediately on a stale signal before the
// watcher's debounce fires for the new event under test, racing with the
// CountEvents check and producing the "No create event received" flake.
func (m *MockLSPClient) ResetEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = []FileEvent{}
	for {
		select {
		case <-m.eventsReceived:
		default:
			return
		}
	}
}

// WaitForEvent waits for at least one event to be received or context to be done
func (m *MockLSPClient) WaitForEvent(ctx context.Context) bool {
	select {
	case <-m.eventsReceived:
		return true
	case <-ctx.Done():
		return false
	}
}

// WaitForEventType waits until an event matching uri+eventType has been
// recorded, or the context is done. This is the race-free variant for tests
// that assert on a specific event: WaitForEvent returns on the *first* signal,
// but a newly-created file produces both a Changed and a Created event (the
// watcher routes the Write-after-Create through NotifyChange because it just
// auto-opened the file). Tests waiting for the Created event must keep
// draining signals until the matching event lands.
func (m *MockLSPClient) WaitForEventType(ctx context.Context, uri string, eventType protocol.FileChangeType) bool {
	for {
		if m.CountEvents(uri, eventType) > 0 {
			return true
		}
		select {
		case <-m.eventsReceived:
			// got a signal; loop back and re-check
		case <-ctx.Done():
			return false
		}
	}
}

// Verify the MockLSPClient implements the watcher.LSPClient interface
var _ watcher.LSPClient = (*MockLSPClient)(nil)
