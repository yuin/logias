package main

import (
	"sync"
)

type dispatcher struct {
	*thread
	exitc chan int

	workers []*worker
}

func newDispathcer(path string) *dispatcher {
	dp := &dispatcher{
		thread: mustNewThread(path, &shared{
			logger: nil,
			quitc:  make(chan *sync.WaitGroup),
		}),
		exitc:   make(chan int),
		workers: []*worker{},
	}
	dp.shared.logger = newLogger(appName, dp.config.LogFile, logLevelOf(dp.config.LogLevel))

	for fpath, _ := range dp.config.Targets {
		dp.workers = append(dp.workers, newWorker(path, fpath, dp.shared))
	}
	return dp
}

func (dp *dispatcher) run() {
	logger := dp.shared.logger
	logger.info("starting logias.")
	for _, worker := range dp.workers {
		go worker.run()
	}
	logger.info("%s", "logias started.")
	for {
		select {
		case <-dp.exitc:
			logger.info("stopping logias.")
			logger.info("waiting for workers.")
			var wg sync.WaitGroup
			wg.Add(len(dp.workers))
			for range dp.workers {
				dp.shared.quitc <- &wg
			}
			wg.Wait()
			logger.info("logias stopped.")
			logger.closeFile()
			return
		}
	}
}
