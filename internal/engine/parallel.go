package engine

import "sync"

// checkWorkers is how many state checks run at once. Each one is usually a
// process of its own (winget list, a PowerShell check), most of the time spent
// waiting on it: run one after the other, the Configurar screen of the gaming
// profile took about half a minute to open.
var checkWorkers = 8

// parallel runs every job, at most n at a time, and returns when all are done.
// The jobs must not depend on each other.
func parallel(n int, jobs []func()) {
	if n < 1 {
		n = 1
	}
	if len(jobs) <= 1 || n == 1 {
		for _, j := range jobs {
			j()
		}
		return
	}
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j func()) {
			defer func() { <-sem; wg.Done() }()
			j()
		}(j)
	}
	wg.Wait()
}
