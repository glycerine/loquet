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
// with Close(closeVal). Close(nil) is also
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
// use Set(). By design, Close(nil) does not change
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

	// how many old versions to retain?
	retain int

	// the current version of closeVal
	version int

	// the old versions, len == retain.
	past []VersionVal[T]
}

// VersionVal is used to store the past
// versions of the closeVal in Val, along with
// their version numbers in Version.
type VersionVal[T any] struct {
	Version int
	Val     *T
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
// our ability ReOpen the Chan after being closed.
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
//
// The retainVersions count provides for
// saving a set of past versions internally.
// It will typically be 0, but can be positive.
// A negative retainVersions will induce a panic.
func NewChan[T any](closeVal *T, retainVersions int) (f *Chan[T]) {
	f = &Chan[T]{
		mut:        sync.Mutex{},
		whenClosed: make(chan struct{}),
		closeVal:   closeVal,
		retain:     retainVersions,
	}
	if f.retain > 0 {
		f.past = make([]VersionVal[T], 0, f.retain+1)
		f.past[0] = VersionVal[T]{Version: f.version, Val: closeVal}
	} else if f.retain < 0 {
		panic("retainVersions cannot be negative")
	}
	return f
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
// Close takes an optional (possibly nil) new closeVal
// value to update the current closeVal (from Set or NewChan).
//
// Close(nil) is fine too, and a no-op. In this case,
// the internal closeVal will
// not be updated. This avoids the Close() calling
// code needing to know about the appropriate closeVal;
// a frequent case when coordinating goroutine shutdown
// from multiple origins.
//
// Close is also a no-op if the Chan is already
// closed. The supplied closeVal is then ignored
// and the internal closeVal will not be updated.
//
// If you need to update the internal closeVal
// without closing the Chan, use Set or SetIfOpen.
//
// To broadcast a new nil value, use SetAndClose().
//
// The returned error will be ErrAlreadyClosed
// if the Chan was already closed; otherwise
// a nil error means that this closeVal was
// stored internally and broadcast.
func (f *Chan[T]) Close(closeVal *T) error {
	f.mut.Lock()
	defer f.mut.Unlock()

	// ensure only Closed once, and
	// keep only the first close msg,
	// so that transaction style cancel/commit
	// can always defer cancel while letting
	// commit first succeed and be preserved.
	if f.isClosed {
		return ErrAlreadyClosed
	}
	f.isClosed = true

	// if closeVal is nil, leave f.closeVal as is.
	// i.e. do not over-ride with nil, because closeVal
	// may be already valid (from NewLoqet or Set).
	if closeVal != nil {
		f.closeVal = closeVal
		f.version++
		if f.retain > 0 {
			f.past = append(f.past, VersionVal[T]{Version: f.version, Val: closeVal})
			if len(f.past) > f.retain {
				f.past = f.past[1:] // discard the oldest
			}
		}
	}
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
	if f.retain > 0 {
		f.past = append(f.past, VersionVal[T]{Version: f.version, Val: closeVal})
		if len(f.past) > f.retain {
			f.past = f.past[1:] // discard the oldest
		}
	}
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
	if f.retain > 0 {
		f.past = append(f.past, VersionVal[T]{Version: f.version, Val: closeVal})
		if len(f.past) > f.retain {
			f.past = f.past[1:] // discard the oldest
		}
	}
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

// ReadVersion is like Read, but also returns
// the version of the closeVal it is returning.
func (f *Chan[T]) ReadVersion() (closeVal *T, isClosed bool, version int) {
	f.mut.Lock()
	closeVal = f.closeVal
	isClosed = f.isClosed
	version = f.version
	f.mut.Unlock()
	return
}

// ReadPast copies the version history into
// the provided slice d, returning the number of history items
// that were copied. The oldest version will
// be at d[0], the most recent (current) version will
// be at d[numCopied-1]. Callers will typically
// want to do d = d[:numCopied] after the call,
// to properly size their d slice.
func (f *Chan[T]) ReadPast(d []VersionVal[T]) (numCopied int) {
	f.mut.Lock()
	defer f.mut.Unlock()
	numCopied = copy(d, f.past)
	return
}

// ReOpen re-opens the Chan, atomically setting
// the supplied closeVal on it. See also Open.
// Calling ReOpen on an already open Chan
// just updates the internal closeVal to the
// one supplied in this call.
func (f *Chan[T]) ReOpen(closeVal *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.closeVal = closeVal
	f.version++
	if f.retain > 0 {
		f.past = append(f.past, VersionVal[T]{Version: f.version, Val: closeVal})
		if len(f.past) > f.retain {
			f.past = f.past[1:] // discard the oldest
		}
	}

	if !f.isClosed {
		return
	}
	f.isClosed = false
	f.whenClosed = make(chan struct{})
}

// Open re-opens the Chan (if it was closed).
// It does not change the closeVal inside.
// If the Chan is already Open, this is a no-op.
func (f *Chan[T]) Open() {
	f.mut.Lock()
	defer f.mut.Unlock()
	if !f.isClosed {
		return
	}
	f.isClosed = false
	f.whenClosed = make(chan struct{})
}

// SetAndClose atomically sets the closeVal
// and closes the Chan. It returns the error
// ErrAlreadyClosed and makes no changes
// if the Chan is already
// closed. As closeVal can be nil, SetAndClose
// allows broadcasting a nil closeVal even
// if NewChan was not called with nil. It
// differs from Close in that Close(nil) does
// not necessarily broadcast a nil, instead
// defaulting to the already set closeVal.
func (f *Chan[T]) SetAndClose(closeVal *T) error {
	f.mut.Lock()
	defer f.mut.Unlock()
	if f.isClosed {
		return ErrAlreadyClosed
	}
	f.closeVal = closeVal
	f.version++
	if f.retain > 0 {
		f.past = append(f.past, VersionVal[T]{Version: f.version, Val: closeVal})
		if len(f.past) > f.retain {
			f.past = f.past[1:] // discard the oldest
		}
	}
	f.isClosed = true
	close(f.whenClosed)
	return nil
}
