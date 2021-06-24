package processor

import (
	"io/fs"
	"log"
	"path/filepath"

	"gitlab.com/hbomb79/TPA/worker"
)

type Processor struct {
	Config TPAConfig
	Queue  ProcessorQueue
}

// Instantiates a new processor by creating the
// bare struct, and loading in the configuration
func New() (proc Processor) {
	proc = Processor{Queue: make(ProcessorQueue, 0)}
	proc.Config.LoadConfig()

	return
}

// Begin will start the processor by installing a file system watcher on the
// input directory, and fills the queue with the files inside the directory
// currently
func (p *Processor) Begin() error {
	pollInboundChan := make(chan interface{})
	pollOutboundChan := make(chan interface{})
	pollingWorker := worker.PollingWorker{
		TickInterval: p.Config.Format.ImportDirTickDelay,
		TaskFn:       p.PollInputSource,
	}

	pollingWorker.TaskFn()
	worker.StartWorkers(1, &pollingWorker, pollInboundChan, pollOutboundChan)

	return nil
}

// PollInputSource will check the source input directory (from p.Config)
// pass along the files it finds to the p.Queue to be inserted if not present.
func (p *Processor) PollInputSource() error {
	log.Printf("Polling input source for new files")
	walkFunc := func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			log.Panicf("PollInputSource failed - %v\n", err.Error())
		}

		if !dir.IsDir() {
			v, err := dir.Info()
			if err != nil {
				log.Panicf("Failed to get FileInfo for path %v - %v\n", path, err.Error())
			}

			p.Queue.HandleFile(path, v)
		}

		return nil
	}

	filepath.WalkDir(p.Config.Format.ImportPath, walkFunc)
	return nil
}
