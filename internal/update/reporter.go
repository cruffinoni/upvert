package update

// Reporter receives progress for one update. Run calls the methods in order:
// Start begins a step, Total sets its optional total size, Advance reports
// cumulative progress, and Finish completes it. The caller must call Abort
// when Run returns an error so an active step can be marked failed.
type Reporter interface {
	Start(step string)
	Total(total int64)
	Advance(done int64)
	Finish(detail string)
	Abort()
}

// nopReporter discards all progress.
type nopReporter struct{}

func (nopReporter) Start(string)  {}
func (nopReporter) Total(int64)   {}
func (nopReporter) Advance(int64) {}
func (nopReporter) Finish(string) {}
func (nopReporter) Abort()        {}
