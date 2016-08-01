// This example demonstrates how to write and read events and event metadata

package main

import (
	"log"
	"net/url"
	"time"

	"github.com/jetbasrawi/goes"
)

// FooEvent is an example event type
//
// An event within your application will just be a plain go struct.
// The event will be serialized and deserialised to JSON and all of
// the standard golang JSON serialisation features apply. Here the
// struct does not use tags to specify a different name for the JSON
// fields or wether a field is optional. The name of the field as it
// is will be used.
type FooEvent struct {
	FooField string
	BarField string
	BazField int
}

func main() {

	// Creating a new Goes client.
	client, err := goes.NewClient(nil, "http://localhost:2113")
	if err != nil {
		log.Fatal(err)
	}

	// Set authentication credentials
	client.SetBasicAuth("admin", "changeit")

	streamName := "foostream"

	// Write some events to the eventstore
	writeEvents(client, streamName)

	// Read the events back
	readEvents(client, streamName)

}

func readEvents(client goes.Client, streamName string) {
	// Create a new stream reader
	reader := client.NewStreamReader(streamName)

	// To begin reading from a stream from a specific version call the
	// NextVersion(int) method.
	// After a call to next, the event returned will be the event at the version
	// specified by NextVersion. In this example code we do not specify a version
	// and the reader uses the default of 0

	// Basic Authentication credentials can be set on the client.
	// Any request made will use these credentials until they are changed.
	// You can change the credentials on a client and any subsequent requests
	// will use the new credentials.

	// By default the first call to next on the stream reader will
	// cue up the event at version 0.
	//
	// To begin reading at a specific version use reader.NextVersion(2)
	//
	// Next() always returns true. The boolean value returned is not intended
	// to provide information about status or control flow. It is simply a
	// convenient mechanism to facilitate an enumeration.
	for reader.Next() {

		// There are a number of errors that can occur when calling Next()
		// All of the events returned are typed so they can all be handled
		// in a consistent manner and can contain supporting data such as
		// the http request and response.
		if reader.Err() != nil {
			switch err := reader.Err().(type) {

			// In the case of network and protocol errors such as when the
			// eventstore server is not online at the url provided a standard
			// *url.Error is returned.
			// In this example we assume that we are waiting for the server to
			// to come online and will respond to this error with a retry after
			// some time.
			// When the eventstore comes online there is a short period where
			// it responds with a StatusServiceUnavailable. The goes client
			// returns a TemporarilyUnavailableError and here we respond to this
			// with a retry after some duration.
			case *url.Error, *goes.TemporarilyUnavailableError:
				<-time.After(time.Duration(30) * time.Second)

			// If a stream is not found a StreamDoesNotExistError is returned.
			// In this example we will retry after some duration expecting that
			// the stream will be created eventually.
			case *goes.NotFoundError:
				<-time.After(time.Duration(10) * time.Second)

			// If the request is not authorised to read from the stream an
			// UnauthorizedError will be returned. For this example we assume
			// that this is not recoverable and we exit.
			case *goes.UnauthorizedError:
				log.Fatal(err)

			// When there are no events returned from the request a NoMoreEventsError is
			// returned. This typically happens when requesting events past the head of
			// the stream.
			// In this example we simply read from start of stream to the end and then
			// exit. See the simulator example for how to poll at the head of the stream.
			case *goes.NoMoreEventsError:
				//Finished reading all the events.
				return

			// Handle any other errors
			default:
				log.Fatal(err)
			}
		} else {

			// If there are no errors, it means the reader has successfully retrieved an event.
			// The following code demonstrates how to deserialise the event data and event meta
			// data returned.

			// The event data and metadata and other useful information about the event
			// are available via the reader's EventResponse() method.

			// One very important piece of data available about the event is the event
			// type. The event type is a string containing the name of the event
			// type and is useful in more realistic scenarios for identifying which event type
			// has been returned and so pass the correct event type to the scan method for
			// deserialization.
			// eventType := reader.EventResponse().Event.EventType

			// Create a new FooEvent to deserialise the event data into
			fooEvent := FooEvent{}

			// Create a map to hold the event metadata
			fooMeta := make(map[string]string)

			// Call scan on the reader passing in pointers to the event and meta data targets
			err := reader.Scan(&fooEvent, &fooMeta)
			// Check for any errors that have occurred during deserialization
			if err != nil {
				log.Fatal(reader.Err())
			}

			log.Printf("\n Event %d returned %#v\n Meta returned %#v\n\n", reader.EventResponse().Event.EventNumber, fooEvent, fooMeta)
		}
	}
}

func writeEvents(client goes.Client, streamName string) {
	// Create some events.
	// Here we will create two events and their associated metadata demonstrating how
	// to write events and event metadata to the eventstore

	// Create an event of type FooEvent
	event1 := &FooEvent{
		FooField: "Lorem Ipsum",
		BarField: "Dolor Sit Amet",
		BazField: 42,
	}

	// Create a map for metadata. This could equally be a struct or anything that can be
	// serialised and deserialised to JSON
	event1Meta := make(map[string]string)
	event1Meta["Foo"] = "consectetur adipiscing elit"

	// Create a goes.Event which contains the event data and metadata.
	//
	// Here the eventID and EventType have been specified explicitly.
	goesEvent1 := goes.ToEventData(goes.NewUUID(), "FooEvent", event1, event1Meta)

	// Create another event of type foo event
	event2 := &FooEvent{
		FooField: "Mary had a little lamb",
		BarField: "It's fleece was white as snow",
		BazField: 1,
	}

	// This time the eventID and EventType have been left blank.
	// The eventID will be an automatically generated uuid.
	// The eventType will be reflected from the type and will be "FooEvent"
	goesEvent2 := goes.ToEventData("", "", event2, nil)

	//Writing to the stream

	// Create a new streamwriter passing in the name of the stream you want to write to.
	// If the stream does not exist, it will be created when events are written to it.
	writer := client.NewStreamWriter(streamName)

	// Write the events to the stream
	// The first argument allows you to specify the expected version. Here expected version
	// is nil and so the events will be appended at the head of the stream regardless of the
	// version of the stream.
	err := writer.Append(nil, goesEvent1, goesEvent2)
	if err != nil {
		log.Fatal(err)
	}

	// Lets repeat this but using an expected version that will cause an error
	// to demonstrate handling concurrency errors
	// This should result in a goes.ConcurrencyError
	v := 0
	err = writer.Append(&v, goesEvent1)
	if err != nil {
		log.Printf("Received expected error. %#v\n", err)
	}

	log.Printf("Written events to the eventStore.")

}
