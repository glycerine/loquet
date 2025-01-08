package loquet_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/glycerine/loquet"
)

type Message struct {
	Err error
}

func doJob(msg *Message, reportBackOn *loquet.Chan[Message]) {

	var err error
	defer reportBackOn.CloseWith(msg)
	//...
	// might return early on error
	if err != nil {
		fmt.Printf("job is erroring out.")
		msg.Err = fmt.Errorf("some error")
		return
	}
	// on success, leave msg.Err == nil
	fmt.Printf("job completed successfully.")
}

func ExampleLoquetChanUse() {

	serviceShutdownCh := make(chan struct{})

	msg := &Message{}
	status := loquet.NewChan[Message](msg)

	go doJob(msg, status)

	// poll for status like this.
	closeVal, isClosed := status.Read()
	fmt.Printf("status.Read() returned isClosed=%v, "+
		"closeVal = '%#v'\n", isClosed, closeVal)

	// wait for close like this.
	select {

	case <-status.WhenClosed():
		latest, isClosed := status.Read()
		if !isClosed {
			panic("(should be) impossible, isClosed should be true if WhenClosed() returns! so we can always latest, _ = status.Read() after WhenClosed()")
		}
		fmt.Printf("status.ReadCh returned "+
			"isClosed=%v, latest = '%#v'\n", isClosed, latest)

	case <-serviceShutdownCh:
		fmt.Printf("good: service shutting down, we were not blocked " +
			"permanently on the <-status.Read() above.\n")
		return
	}

}

func TestOpenStopsClose(t *testing.T) {

	ch := loquet.NewChan[int](nil)
	insideLoop := make(chan bool)
	goroDone := make(chan bool)

	go func() {
		defer close(goroDone)
		isClosed := false
		for i := 0; true; i++ {
			if isClosed {
				ch.Open()
				fmt.Printf("Open()-ed again!\n")
				isClosed = false
			}

			fmt.Printf("about to block on WhenClosed\n")
			if i == 0 {
				close(insideLoop)
			}
			select {
			case <-ch.WhenClosed():
				fmt.Printf("closed!\n")
				isClosed = true
			case <-time.After(time.Second):
				fmt.Printf("one second timeout waiting for close... returning\n")
				return
			}
		}
	}()
	<-insideLoop
	ch.Close()
	<-goroDone
}
