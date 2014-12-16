package raven

import (
	"fmt"
	"log"
	"net/http"
)

func Example() {
	// ... i.e. raisedErr is incoming error
	var raisedErr error
	// sentry DSN generated by Sentry server
	var sentryDSN string
	// r is a request performed when error occured
	var r *http.Request
	client, err := NewClient(sentryDSN, nil)
	if err != nil {
		log.Fatal(err)
	}
	trace := NewStacktrace(0, 2, nil)
	eventID, ch := client.CaptureError(raisedErr, &Context{
		Interfaces: []Interface{NewException(raisedErr, trace), NewHttp(r)},
	})
	if err = <-ch; err != nil {
		log.Fatal(err)
	}
	message := fmt.Sprintf("Captured error with id %s: %q", eventID, raisedErr)
	log.Println(message)
}
