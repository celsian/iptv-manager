package autosearch

import (
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron     *cron.Cron
	executor *Executor
	store    *Store
	mu       sync.Mutex
	jobIDs   map[string]cron.EntryID // maps job ID to cron entry ID
}

func NewScheduler(executor *Executor, store *Store) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		executor: executor,
		store:    store,
		jobIDs:   make(map[string]cron.EntryID),
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
	s.ReloadJobs()
	log.Println("AutoSearch: Scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("AutoSearch: Scheduler stopped")
}

func (s *Scheduler) ReloadJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove all existing cron entries
	for jobID, entryID := range s.jobIDs {
		s.cron.Remove(entryID)
		delete(s.jobIDs, jobID)
	}

	// Add enabled jobs
	jobs := s.store.GetEnabledJobs()
	for _, job := range jobs {
		s.scheduleJob(job)
	}

	log.Printf("AutoSearch: Loaded %d scheduled jobs", len(jobs))
}

func (s *Scheduler) scheduleJob(job *Job) {
	if job.Schedule == "" {
		log.Printf("AutoSearch: Job %s has no schedule, skipping", job.Name)
		return
	}

	jobID := job.ID
	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		log.Printf("AutoSearch: Executing scheduled job %s", jobID)
		s.executor.ExecuteJob(jobID)
	})

	if err != nil {
		log.Printf("AutoSearch: Failed to schedule job %s: %v", job.Name, err)
		return
	}

	s.jobIDs[job.ID] = entryID
	log.Printf("AutoSearch: Scheduled job %s with schedule %s", job.Name, job.Schedule)
}

func (s *Scheduler) AddJob(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing entry if present
	if entryID, exists := s.jobIDs[job.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobIDs, job.ID)
	}

	if job.Enabled {
		s.scheduleJob(job)
	}
}

func (s *Scheduler) RemoveJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.jobIDs[jobID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobIDs, jobID)
		log.Printf("AutoSearch: Removed job %s from scheduler", jobID)
	}
}

func (s *Scheduler) RunJobNow(jobID string) ExecutionResult {
	return s.executor.ExecuteJob(jobID)
}
