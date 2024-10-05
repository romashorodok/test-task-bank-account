package backoff

import (
	"context"
	"math/rand/v2"
	"time"
)

type Clock interface {
	Now() time.Time
}

var _ Clock = (*systemClock)(nil)

type systemClock struct{}

func (t systemClock) Now() time.Time {
	return time.Now()
}

var SystemClock = systemClock{}

type Backoff struct {
	InitialInterval     time.Duration
	RandomizationFactor float64
	Multiplier          float64
	MaxInterval         time.Duration

	// After MaxElapsedTime the Backoff stops.
	// It never stops if MaxElapsedTime == 0.
	MaxElapsedTime time.Duration
	Clock          Clock

	currentInterval time.Duration
	startTime       time.Time
}

// Reset the interval back to the initial retry interval and restarts the timer.
// Reset must be called before using b.
func (b *Backoff) Reset() {
	b.currentInterval = b.InitialInterval
	b.startTime = b.Clock.Now()
}

// Returns a random value from the following interval:
//
//	[randomizationFactor * currentInterval, randomizationFactor * currentInterval].
func getRandomValueFromInterval(randomizationFactor, random float64, currentInterval time.Duration) time.Duration {
	delta := randomizationFactor * float64(currentInterval)
	minInterval := float64(currentInterval) - delta
	maxInterval := float64(currentInterval) + delta

	// Get a random value from the range [minInterval, maxInterval].
	// The formula used below has a +1 because if the minInterval is 1 and the maxInterval is 3 then
	// we want a 33% chance for selecting either 1, 2 or 3.
	return time.Duration(minInterval + (random * (maxInterval - minInterval + 1)))
}

const Stop time.Duration = -1

// NextBackOff calculates the next backoff interval using the formula:
//
//	Randomized interval = RetryInterval * (1 ± RandomizationFactor)
func (b *Backoff) NextBackOff() time.Duration {
	// Make sure we have not gone over the maximum elapsed time.
	if b.MaxElapsedTime != 0 && b.GetElapsedTime() > b.MaxElapsedTime {
		return Stop
	}
	defer b.incrementCurrentInterval()
	return getRandomValueFromInterval(b.RandomizationFactor, rand.Float64(), b.currentInterval)
}

// GetElapsedTime returns the elapsed time since an Backoff instance
// is created and is reset when Reset() is called.
//
// The elapsed time is computed using time.Now().UnixNano(). It is
// safe to call even while the backoff policy is used by a running
// ticker.
func (b *Backoff) GetElapsedTime() time.Duration {
	return b.Clock.Now().Sub(b.startTime)
}

// Increments the current interval by multiplying it with the multiplier.
func (b *Backoff) incrementCurrentInterval() {
	// Check for overflow, if overflow is detected set the current interval to the max interval.
	if float64(b.currentInterval) >= float64(b.MaxInterval)/b.Multiplier {
		b.currentInterval = b.MaxInterval
	} else {
		b.currentInterval = time.Duration(float64(b.currentInterval) * b.Multiplier)
	}
}

const (
	BACKOFF_INITIAL_INTERVAL     = 500 * time.Millisecond
	BACKOFF_RANDOMIZATION_FACTOR = 0.5
	BACKOFF_MULTIPLIER           = 1.5
	BACKOFF_MAX_INTERVAL         = 60 * time.Second
)

func NewBackoffDefault() Backoff {
	return Backoff{
		InitialInterval:     BACKOFF_INITIAL_INTERVAL,
		RandomizationFactor: BACKOFF_RANDOMIZATION_FACTOR,
		Multiplier:          BACKOFF_MULTIPLIER,
		MaxInterval:         BACKOFF_MAX_INTERVAL,
		MaxElapsedTime:      0, // no support for disabling reconnect, only close of Pub/Sub can stop reconnecting
		Clock:               SystemClock,
	}
}

type Timer interface {
	Start(duration time.Duration)
	Stop()
	C() <-chan time.Time
}

// defaultTimer implements Timer interface using time.Timer
type defaultTimer struct {
	timer *time.Timer
}

// C returns the timers channel which receives the current time when the timer fires.
func (t *defaultTimer) C() <-chan time.Time {
	return t.timer.C
}

// Start starts the timer to fire after the given duration
func (t *defaultTimer) Start(duration time.Duration) {
	if t.timer == nil {
		t.timer = time.NewTimer(duration)
	} else {
		t.timer.Reset(duration)
	}
}

// Stop is called when the timer is not used anymore and resources may be freed.
func (t *defaultTimer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
	}
}

func Retry(cb func() error, backoff Backoff) (err error) {
	timer := &defaultTimer{}
	defer timer.Stop()

	var next time.Duration

	ctx := context.Background()
	backoff.Reset()
	for {
		if err = cb(); err == nil {
			return nil
		}

		if next = backoff.NextBackOff(); next == Stop {
			return err
		}

		timer.Start(next)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C():
		}
	}
}
