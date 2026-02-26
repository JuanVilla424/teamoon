package jobs

import (
	"context"
	"log"
	"time"
)

// RunFn is called when a job fires. It receives the job and should execute it.
type RunFn func(Job)

// StartScheduler starts a background goroutine that checks enabled jobs every minute
// and fires RunFn when a job's cron schedule matches.
func StartScheduler(ctx context.Context, runFn RunFn) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("[jobs] scheduler stopped")
				return
			case t := <-ticker.C:
				checkAndRun(t, runFn)
			}
		}
	}()
	log.Println("[jobs] scheduler started")
}

func checkAndRun(t time.Time, runFn RunFn) {
	all, err := ListAll()
	if err != nil {
		return
	}
	for _, j := range all {
		if !j.Enabled || j.Status == StatusRunning {
			continue
		}
		if MatchesCron(j.Schedule, t) {
			log.Printf("[jobs] firing job #%d %q", j.ID, j.Name)
			go runFn(j)
		}
	}
}
