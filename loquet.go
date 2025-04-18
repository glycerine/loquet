package loquet

import (
	"fmt"
	"sync"
)

var ErrAlreadyClosed = fmt.Errorf("the loquet.Chan is already closed.")

// Chan encapsulates in one convenient
// place several common patterns that
// Go developers often find need of.
//
// The loquet.Chan is similar to a Go channel, in that
// it conveys a value to readers after an
// event, but offers two principal advantages
// in the common case where there is only
// one such event, namely the close of the channel.
//
// First, Chan offers an idempotent Close method.
// This is already a massive improvement over
// raw Go channels. Second, on top of that,
// we add the ability of the Close method
// to convey a value, one that is not just
// the zero-value. We call this transmitted
// value the closeVal in the details that follow.
//
// The trade-off here is that Chan does not
// offer "send" semantics, only enhanced "close" semantics.
// Unlike standard Go channels, a Chan
// is not useful for queuing up many items to
// be read, nor for assigning a series of
// items each to a single reader. Instead,
// Chan significantly enhances the broadcast
// capabilities inherent in closing a channel.
//
// Users obtain the closeVal via the Chan.Read()
// method. Chan.Read() can be called before or after
// the Chan is closed, and will return
// the current closeVal and an isClosed flag
// indicating if the Chan has been closed or not.
//
// To broadcast a value over a Chan, users can
// establish or update the closeVal
// (that value to-be-read by Read) in
// three different ways:
//
// 1) The closeVal may naturally be set during creation,
// by supplying it to the NewChan(closeVal) call.
//
// 2) The closeVal can also be set during closing
// with CloseWith(closeVal). Close() is also
// valid but will not alter an established closeVal.
// This simplifies the code that closes the channel.
// It can remain blissfully unaware of the appropriate
// closeVal if it has already been set correctly.
//
// Chan.Close() will close the WhenClosed() returned channel,
// if it has not already been closed.
// Thus it is safe to call Close() multiple times, knowing
// this will not result in a panic. The underlying
// WhenClosed() channel is only ever closed
// once.
//
// 3) Lastly, the closeVal can also be updated with
// the Set(closeVal) method. Set will not change the
// closed/open status of the Chan. If you wish
// to nil out an established closeVal, you must
// use Set(). By design, Close() does not change
// the closeVal. Set on a closed Chan is valid,
// but there is then no universal guarantee that
// all Read() calls will obtain the
// same closeVal. If all updates use SetIfOpen
// instead, then this guarantee does apply after
// a Close, since SetIfOpen is a no-op on a closed Chan.
//
// The closeVal available from Read() is
// independent of the open/closed status of the Chan,
// and it is up to the user to assign meaning
// to the closeVal, isClosed := Read() values received.
//
// A call to NewChan() is required to produce a new Chan.
// Although the zero-value of a Chan is currently viable,
// it contains a sync.Mutex, and so cannot be
// copied after first use anyway. Moreover we want to
// preserve our ability to change this in the future;
// to make the zero-value not viable if it improves
// the design or efficiency of use.
//
// For these reasons, and to keep the shared internal
// state correct, user code should always deal with
// *Chan pointers. Passing a Chan by value instead
// of by a *Chan pointer will result in incorrect,
// undefined behavior.
//
// Notice that the generic parameter is a T in Chan[T], but
// all operations deal in *T. For example, to work
// with a closeVal of type *Message,
// simply call NewChan[Message](closeVal *Message).
type Chan[T any] struct {
	mut sync.Mutex

	whenClosed chan struct{}

	// closeVal and isClosed are the values that
	// we report from Read().
	closeVal *T
	isClosed bool
	version  int64
}

// WhenClosed returns a channel that
// is closed exactly once on the
// first Chan.Close() call.
//
// Users typically call Chan.Read() after noting
// that the WhenClosed channel has been closed in order
// to retreive the current closeVal. This two-step
// process of notification (on a channel) followed by
// a Read is needed because a closed Go
// channel only returns the zero value; Chan was
// created to work around this limitation.
// Users must never close() the WhenClosed() channel
// themselves.
//
// Users should never store the received channel;
// instead they should always call WhenClosed()
// on the right hand side of a channel operation,
// just in time when they need. Doing so preserves
// our ability to update the internal channel in
// the future if that becomes useful.
//
// ~~~
//
//	select {
//	    case <-myLoquetChan.WhenClosed():
//	        val, _ := myLoqetChan.Read()
//
// ~~~
func (f *Chan[T]) WhenClosed() <-chan struct{} {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.whenClosed
}

// NewChan creates a new Chan, given a type T.
// Notice that the generic parameter is a T in Chan[T], but
// all operations deal in *T. For example, if you have
// `var closeVal *Message = &Message{}`, then
// simply call `NewChan[Message](closeVal)`.
func NewChan[T any](closeVal *T) (f *Chan[T]) {
	f = &Chan[T]{
		mut:        sync.Mutex{},
		whenClosed: make(chan struct{}),
		closeVal:   closeVal,
	}
	return
}

// CloseWith provides an idempotent close of the
// WhenClosed channel. Multiple calls to CloseWith
// will result in only a single close of
// WhenClosed.
// This addresses a major design limitation of Go channels.
// By using a Chan instead of a raw Go channel,
// you need not worry about panic from repeated
// closing.
//
// CloseWith takes a new closeVal to broadcast.
//
// CloseWith is a no-op if the Chan is already
// closed. The supplied closeVal is then ignored
// and the internal closeVal will not be updated.
//
// If you need to update the internal closeVal
// without closing the Chan, use Set or SetIfOpen.
//
// The returned error will be ErrAlreadyClosed
// if the Chan was already closed; otherwise
// a nil error means that this closeVal was
// stored internally and broadcast.
func (f *Chan[T]) CloseWith(closeVal *T) error {
	f.mut.Lock()
	defer f.mut.Unlock()

	if f.isClosed {
		return ErrAlreadyClosed
	}
	f.isClosed = true
	f.closeVal = closeVal
	f.version++
	close(f.whenClosed)
	return nil
}

// Close provides an idempotent close of the
// WhenClosed channel. Multiple calls to Close
// will result in only a single close of
// WhenClosed.
// This addresses a major design limitation of Go channels.
// By using a Chan instead of a raw Go channel,
// you need not worry about panic from repeated
// closing.
//
// Close is also a no-op if the Chan is already
// closed.
//
// The returned error will be ErrAlreadyClosed
// if the Chan was already closed; otherwise
// a nil error means that the WhenClosed
// channel was closed and the internal closeVal
// will be broadcast to Read() callers.
func (f *Chan[T]) Close() error {
	f.mut.Lock()
	defer f.mut.Unlock()

	if f.isClosed {
		return ErrAlreadyClosed
	}
	f.isClosed = true
	close(f.whenClosed)
	return nil
}

// Set changes the closeVal without
// actually closing the Chan (compare to Close).
// That is, Set will change the closeVal no
// matter if the Chan is open or close.
//
// The previously set closeVal is returned in old;
// but this may commonly be ignored.
//
// Use SetIfOpen to set a new closeVal only
// if the Chan is still open.
func (f *Chan[T]) Set(closeVal *T) (old *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	old = f.closeVal
	f.closeVal = closeVal
	f.version++
	return
}

// SetIfOpen is a no-op if the Chan is closed.
// Otherwise, it behaves like Set().
// SetIfOpen will still return the
// current internal closeVal in old even if
// it was not updated due to the Chan
// being closed.
func (f *Chan[T]) SetIfOpen(closeVal *T) (old *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	old = f.closeVal
	if f.isClosed {
		return
	}
	f.closeVal = closeVal
	f.version++
	return
}

// Read returns the current closeVal and the
// isClosed status.
//
// Note that an open channel may supply
// a valid closeVal; but this determined by
// the user. For example, the user may
// have initialized the closeVal during NewChan(closeVal),
// or may have subsequently called Set(closeVal).
//
// To avoid busy waiting, use the WhenClosed channel
// in your select statements, and
// then call Read() once it is closed.
//
/* For example:
~~~
 ...
 msg := &Message{}
 status := loquet.NewChan[Message](msg)
 ...
 // to poll the status Chan:
 // (and because val may be useful even before closing)
 val, isClosed := status.Read()
     ...
 // to wait for the status Chan to be closed:
 select {
 case <-status.WhenClosed():
	   val, isClosed := status.Read()
      ... react to val... (isClosed will always be true here).
~~~
*/
func (f *Chan[T]) Read() (closeVal *T, isClosed bool) {
	f.mut.Lock()
	closeVal = f.closeVal
	isClosed = f.isClosed
	f.mut.Unlock()
	return
}

// ReadVersionAndReset returns the current closeVal and
// its version number, and atomically replaces the
// internal closeVal with newCloseVal. This allows a
// single reader to notice if they have missed any
// version updates due to a race, and thereful do a full check
// on state rather than the usual cheap only-check
// state when the Chan is closed.
func (f *Chan[T]) ReadVersionAndReset(newCloseVal *T) (closeVal *T, version int64) {
	f.mut.Lock()
	closeVal = f.closeVal
	version = f.version

	f.isClosed = false
	f.closeVal = newCloseVal
	f.version++
	f.mut.Unlock()
	return
}

// ReadAndReset is the same as ReadVersionAndReset, except
// that it doesn't return the version of the returned closeVal.
func (f *Chan[T]) ReadAndReset(newCloseVal *T) (closeVal *T) {
	f.mut.Lock()
	closeVal = f.closeVal

	f.isClosed = false
	f.closeVal = newCloseVal
	f.version++
	f.mut.Unlock()
	return
}
