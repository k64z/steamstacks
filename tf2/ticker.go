package tf2

import "time"

const helloInterval = 5 * time.Second

// ticker abstracts time.Ticker for testing.
type ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct{ t *time.Ticker }

func (r *realTicker) C() <-chan time.Time { return r.t.C }
func (r *realTicker) Stop()              { r.t.Stop() }

// defaultNewTicker is the production constructor for the hello-resend
// ticker; tests swap Client.newTicker for a fake.
func defaultNewTicker(d time.Duration) ticker {
	return &realTicker{t: time.NewTicker(d)}
}
