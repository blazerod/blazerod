package v8engine

// #include <stdlib.h>
// #include "v8engine.h"
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

var v8init sync.Once

// Engine is a standalone instance of the V8 engine (isolate + context)
type Engine struct {
	contextPtr C.ContextPtr
}

// NewEngine creates a new V8 engine (isolate + context)
func NewEngine() *Engine {
	v8init.Do(func() {
		C.InitV8()
	})

	contextPtr := C.NewContext()

	engine := &Engine{
		contextPtr: contextPtr,
	}

	runtime.SetFinalizer(engine, (*Engine).finalizer)

	return engine
}

// Run executes a script in the engine, returning the result
func (e *Engine) Run(source string, origin string) (*Value, error) {
	cSource := C.CString(source)
	cOrigin := C.CString(origin)
	defer C.free(unsafe.Pointer(cSource))
	defer C.free(unsafe.Pointer(cOrigin))

	rtn := C.Run(e.contextPtr, cSource, cOrigin)
	return getValue(rtn), getError(rtn)
}

// LoadModule executes a script in the engine, returning the result
func (e *Engine) LoadModule(source string, origin string, resolve ModuleResolverCallback) int {
	cSource := C.CString(source)
	cOrigin := C.CString(origin)
	defer C.free(unsafe.Pointer(cSource))
	defer C.free(unsafe.Pointer(cOrigin))

	resolverTableLock.Lock()
	nextResolverToken++
	token := nextResolverToken
	resolverFuncs[token] = resolve
	resolverTableLock.Unlock()

	cToken := C.int(token)

	rtn := C.LoadModule(e.contextPtr, cSource, cOrigin, cToken)

	resolverTableLock.Lock()
	delete(resolverFuncs, token)
	resolverTableLock.Unlock()

	return int(rtn)
}

// Send sends bytes to V8
func (e *Engine) Send(msg []byte) error {
	msgPointer := C.CBytes(msg)

	code := C.Send(e.contextPtr, C.size_t(len(msg)), msgPointer)
	if code != 0 {
		return fmt.Errorf("expected 0, got %d", code)
	}
	return nil
}

func (e *Engine) finalizer() {
	C.DisposeContext(e.contextPtr)
	e.contextPtr = nil

	runtime.SetFinalizer(e, nil)
}

func getValue(rtn C.RtnValue) *Value {
	if rtn.value == nil {
		return nil
	}
	v := &Value{rtn.value}
	runtime.SetFinalizer(v, (*Value).finalizer)
	return v
}

func getError(rtn C.RtnValue) error {
	if rtn.error.msg == nil {
		return nil
	}
	err := &JSError{
		Message:    C.GoString(rtn.error.msg),
		Location:   C.GoString(rtn.error.location),
		StackTrace: C.GoString(rtn.error.stack),
	}
	C.free(unsafe.Pointer(rtn.error.msg))
	C.free(unsafe.Pointer(rtn.error.location))
	C.free(unsafe.Pointer(rtn.error.stack))
	return err
}
