package v8go

// #include <stdlib.h>
// #include "v8go.h"
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// InspectorOptions configures a V8 Inspector context/session.
type InspectorOptions struct {
	ContextGroupID int
	Name           string
	Origin         string
	WaitForDebugger bool
}

// InspectorChannel receives Chrome DevTools Protocol messages from V8.
type InspectorChannel interface {
	SendResponse(callID int, message []byte)
	SendNotification(message []byte)
	Flush()
}

// Inspector wraps V8's native inspector for one isolate.
type Inspector struct {
	ptr    C.InspectorPtr
	iso    *Isolate
	mu     sync.Mutex
	sessions map[int]*InspectorSession
	closed bool
}

// InspectorSession is one CDP session connected to an Inspector context group.
type InspectorSession struct {
	ptr        C.SessionPtr
	ins        *Inspector
	channelRef int
	closed     bool
}

var inspectorChannels = struct {
	sync.RWMutex
	next int
	data map[int]InspectorChannel
}{data: map[int]InspectorChannel{}}

// NewInspector creates a V8 Inspector bound to one isolate.
func NewInspector(iso *Isolate) (*Inspector, error) {
	if iso == nil || iso.ptr == nil {
		return nil, errors.New("v8go: inspector requires a live isolate")
	}

	ptr := C.InspectorNew(iso.ptr)
	if ptr == nil {
		return nil, errors.New("v8go: inspector creation failed")
	}
	return &Inspector{ptr: ptr, iso: iso, sessions: map[int]*InspectorSession{}}, nil
}

// NotifyContextCreated registers a context with V8 Inspector.
func (ins *Inspector) NotifyContextCreated(ctx *Context, opt InspectorOptions) error {
	if err := ins.checkLive(); err != nil {
		return err
	}
	if ctx == nil || ctx.ptr == nil {
		return errors.New("v8go: inspector contextCreated requires a live context")
	}

	groupID := normalizeContextGroupID(opt.ContextGroupID)
	name := C.CString(opt.Name)
	origin := C.CString(opt.Origin)
	defer C.free(unsafe.Pointer(name))
	defer C.free(unsafe.Pointer(origin))

	return inspectorError(C.InspectorContextCreated(ins.ptr, ctx.ptr, C.int(groupID), name, origin))
}

// NotifyContextDestroyed unregisters a context from V8 Inspector.
func (ins *Inspector) NotifyContextDestroyed(ctx *Context) error {
	if err := ins.checkLive(); err != nil {
		return err
	}
	if ctx == nil || ctx.ptr == nil {
		return nil
	}
	return inspectorError(C.InspectorContextDestroyed(ins.ptr, ctx.ptr))
}

// Connect creates a CDP session for a context.
func (ins *Inspector) Connect(ctx *Context, ch InspectorChannel, opt InspectorOptions) (*InspectorSession, error) {
	if err := ins.checkLive(); err != nil {
		return nil, err
	}
	if ctx == nil || ctx.ptr == nil {
		return nil, errors.New("v8go: inspector connect requires a live context")
	}
	if ch == nil {
		return nil, errors.New("v8go: inspector connect requires a channel")
	}

	groupID := normalizeContextGroupID(opt.ContextGroupID)
	channelRef := registerInspectorChannel(ch)
	wait := 0
	if opt.WaitForDebugger {
		wait = 1
	}
	ptr := C.InspectorConnect(ins.ptr, ctx.ptr, C.int(groupID), C.int(channelRef), C.int(wait))
	if ptr == nil {
		unregisterInspectorChannel(channelRef)
		return nil, errors.New("v8go: inspector connect failed")
	}

	session := &InspectorSession{ptr: ptr, ins: ins, channelRef: channelRef}
	ins.mu.Lock()
	if ins.closed {
		ins.mu.Unlock()
		C.InspectorSessionClose(ptr)
		unregisterInspectorChannel(channelRef)
		return nil, errors.New("v8go: inspector is closed")
	}
	if ins.sessions == nil {
		ins.sessions = map[int]*InspectorSession{}
	}
	ins.sessions[channelRef] = session
	ins.mu.Unlock()
	return session, nil
}

// Close releases the inspector and all native sessions it still owns.
func (ins *Inspector) Close() error {
	if ins == nil {
		return nil
	}
	ins.mu.Lock()
	if ins.closed {
		ins.mu.Unlock()
		return nil
	}
	ptr := ins.ptr
	sessions := make([]*InspectorSession, 0, len(ins.sessions))
	for _, session := range ins.sessions {
		sessions = append(sessions, session)
	}
	ins.sessions = nil
	ins.ptr = nil
	ins.closed = true
	ins.mu.Unlock()

	for _, session := range sessions {
		if session != nil {
			_ = session.Close()
		}
	}
	if ptr != nil {
		C.InspectorFree(ptr)
	}
	return nil
}

// Dispatch sends one raw CDP JSON message into the session.
func (s *InspectorSession) Dispatch(message []byte) error {
	if err := s.checkLive(); err != nil {
		return err
	}
	if len(message) == 0 {
		return errors.New("v8go: inspector dispatch requires a non-empty message")
	}
	return inspectorError(C.InspectorDispatch(s.ptr, (*C.char)(unsafe.Pointer(&message[0])), C.int(len(message))))
}

// Pause requests a debugger pause.
func (s *InspectorSession) Pause(reason string) error {
	if err := s.checkLive(); err != nil {
		return err
	}
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	return inspectorError(C.InspectorPause(s.ptr, cReason))
}

// Resume resumes debugger execution.
func (s *InspectorSession) Resume() error {
	if err := s.checkLive(); err != nil {
		return err
	}
	return inspectorError(C.InspectorResume(s.ptr))
}

// Stop prepares the debugger session for shutdown.
func (s *InspectorSession) Stop() error {
	if err := s.checkLive(); err != nil {
		return err
	}
	return inspectorError(C.InspectorStop(s.ptr))
}

// State returns V8 Inspector's serialized session state.
func (s *InspectorSession) State() []byte {
	if err := s.checkLive(); err != nil {
		return nil
	}
	rtn := C.InspectorState(s.ptr)
	if rtn.error.msg != nil {
		freeRtnString(rtn)
		return nil
	}
	defer freeRtnString(rtn)
	if rtn.data == nil || rtn.length == 0 {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(rtn.data), rtn.length)
}

// Close releases the native inspector session and unregisters its Go channel.
func (s *InspectorSession) Close() error {
	if s == nil || s.closed {
		return nil
	}
	ins := s.ins
	ptr := s.ptr
	channelRef := s.channelRef
	s.ptr = nil
	s.closed = true
	if ins != nil {
		ins.mu.Lock()
		if ins.sessions != nil {
			delete(ins.sessions, channelRef)
		}
		ins.mu.Unlock()
	}
	if ptr != nil {
		C.InspectorSessionClose(ptr)
	}
	unregisterInspectorChannel(channelRef)
	return nil
}

func (ins *Inspector) checkLive() error {
	if ins == nil {
		return errors.New("v8go: inspector is nil")
	}
	ins.mu.Lock()
	defer ins.mu.Unlock()
	if ins.closed || ins.ptr == nil {
		return errors.New("v8go: inspector is closed")
	}
	return nil
}

func (s *InspectorSession) checkLive() error {
	if s == nil || s.closed || s.ptr == nil {
		return errors.New("v8go: inspector session is closed")
	}
	return nil
}

func normalizeContextGroupID(id int) int {
	if id == 0 {
		return 1
	}
	return id
}

func inspectorError(rtn C.RtnError) error {
	if rtn.msg == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(rtn.msg))
	if rtn.location != nil {
		defer C.free(unsafe.Pointer(rtn.location))
	}
	if rtn.stack != nil {
		defer C.free(unsafe.Pointer(rtn.stack))
	}
	return fmt.Errorf("v8go: inspector: %s", C.GoString(rtn.msg))
}

func freeRtnString(rtn C.RtnString) {
	if rtn.data != nil {
		C.free(unsafe.Pointer(rtn.data))
	}
	if rtn.error.msg != nil {
		C.free(unsafe.Pointer(rtn.error.msg))
	}
	if rtn.error.location != nil {
		C.free(unsafe.Pointer(rtn.error.location))
	}
	if rtn.error.stack != nil {
		C.free(unsafe.Pointer(rtn.error.stack))
	}
}

func registerInspectorChannel(ch InspectorChannel) int {
	inspectorChannels.Lock()
	defer inspectorChannels.Unlock()
	inspectorChannels.next++
	ref := inspectorChannels.next
	inspectorChannels.data[ref] = ch
	return ref
}

func unregisterInspectorChannel(ref int) {
	if ref == 0 {
		return
	}
	inspectorChannels.Lock()
	delete(inspectorChannels.data, ref)
	inspectorChannels.Unlock()
}

func getInspectorChannel(ref int) InspectorChannel {
	inspectorChannels.RLock()
	ch := inspectorChannels.data[ref]
	inspectorChannels.RUnlock()
	return ch
}

//export goInspectorSendResponse
func goInspectorSendResponse(channelRef C.int, callID C.int, message *C.char, length C.int) {
	ch := getInspectorChannel(int(channelRef))
	if ch == nil || message == nil || length == 0 {
		return
	}
	ch.SendResponse(int(callID), C.GoBytes(unsafe.Pointer(message), length))
}

//export goInspectorSendNotification
func goInspectorSendNotification(channelRef C.int, message *C.char, length C.int) {
	ch := getInspectorChannel(int(channelRef))
	if ch == nil || message == nil || length == 0 {
		return
	}
	ch.SendNotification(C.GoBytes(unsafe.Pointer(message), length))
}

//export goInspectorFlush
func goInspectorFlush(channelRef C.int) {
	ch := getInspectorChannel(int(channelRef))
	if ch == nil {
		return
	}
	ch.Flush()
}
