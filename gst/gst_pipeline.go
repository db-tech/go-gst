package gst

/*
#cgo pkg-config: gstreamer-1.0
#cgo CFLAGS: -Wno-deprecated-declarations -Wno-incompatible-pointer-types -g
#include <gst/gst.h>
#include "gst.go.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/gotk3/gotk3/glib"
)

// Pipeline is a go implementation of a GstPipeline. Helper methods are provided for constructing
// pipelines either using file descriptors or the Appsrc/Appsink APIs. The struct itself implements
// a ReadWriteCloser.
type Pipeline struct{ *Bin }

// NewPipeline allocates and returns a new empty pipeline. If name is empty, one
// is generated by gstreamer.
func NewPipeline(name string) (*Pipeline, error) {
	var cChar *C.char
	if name != "" {
		cChar = C.CString(name)
		defer C.free(unsafe.Pointer(cChar))
	}
	pipeline := C.gst_pipeline_new((*C.gchar)(cChar))
	if pipeline == nil {
		return nil, errors.New("Could not create new pipeline")
	}
	return wrapPipeline(glib.Take(unsafe.Pointer(pipeline))), nil
}

// NewPipelineFromString creates a new gstreamer pipeline from the given launch string.
func NewPipelineFromString(launchv string) (*Pipeline, error) {
	if len(strings.Split(launchv, "!")) < 2 {
		return nil, fmt.Errorf("Given string is too short for a pipeline: %s", launchv)
	}
	cLaunchv := C.CString(launchv)
	defer C.free(unsafe.Pointer(cLaunchv))
	var gerr *C.GError
	pipeline := C.gst_parse_launch((*C.gchar)(cLaunchv), (**C.GError)(&gerr))
	if gerr != nil {
		defer C.g_error_free((*C.GError)(gerr))
		errMsg := C.GoString(gerr.message)
		return nil, errors.New(errMsg)
	}
	return wrapPipeline(glib.Take(unsafe.Pointer(pipeline))), nil
}

// Instance returns the native GstPipeline instance.
func (p *Pipeline) Instance() *C.GstPipeline { return C.toGstPipeline(p.unsafe()) }

// GetBus returns the message bus for this pipeline.
func (p *Pipeline) GetBus() *Bus {
	cBus := C.gst_pipeline_get_bus((*C.GstPipeline)(p.Instance()))
	return wrapBus(glib.Take(unsafe.Pointer(cBus)))
}

// GetPipelineClock returns the global clock for this pipeline.
func (p *Pipeline) GetPipelineClock() *Clock {
	cClock := C.gst_pipeline_get_pipeline_clock((*C.GstPipeline)(p.Instance()))
	return wrapClock(glib.Take(unsafe.Pointer(cClock)))
}

/*
SetAutoFlushBus can be used to disable automatically flushing the message bus
when a pipeline goes to StateNull.

Usually, when a pipeline goes from READY to NULL state, it automatically flushes
all pending messages on the bus, which is done for refcounting purposes, to break
circular references.

This means that applications that update state using (async) bus messages (e.g. do
certain things when a pipeline goes from PAUSED to READY) might not get to see
messages when the pipeline is shut down, because they might be flushed before they
can be dispatched in the main thread. This behaviour can be disabled using this function.

It is important that all messages on the bus are handled when the automatic flushing
is disabled else memory leaks will be introduced.
*/
func (p *Pipeline) SetAutoFlushBus(b bool) {
	C.gst_pipeline_set_auto_flush_bus(p.Instance(), gboolean(b))
}

// Start is the equivalent to calling SetState(StatePlaying) on the underlying GstElement.
func (p *Pipeline) Start() error {
	return p.SetState(StatePlaying)
}

// Destroy will attempt to stop the pipeline and then unref once the stream has
// fully completed.
func (p *Pipeline) Destroy() error {
	if err := p.BlockSetState(StateNull); err != nil {
		return err
	}
	p.Unref()
	return nil
}

// Wait waits for the given pipeline to reach end of stream or be stopped.
func Wait(p *Pipeline) {
	if p.Instance() == nil {
		return
	}
	msgCh := p.GetBus().MessageChan()
	for {
		select {
		default:
			if p.Instance() == nil || p.GetState() == StateNull {
				return
			}
		case msg := <-msgCh:
			defer msg.Unref()
			switch msg.Type() {
			case MessageEOS:
				return
			}
		}
	}
}