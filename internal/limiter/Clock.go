package limiter

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

type FakeClock struct {
	T time.Time
}

func (f *FakeClock) Now() time.Time {
	return f.T
}

func (f *FakeClock) Advance(d time.Duration) {
	f.T = f.T.Add(d)
}
