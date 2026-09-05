package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"docker-panel/internal/agent"
	"docker-panel/internal/database"
	"docker-panel/internal/models"
)

var (
	ErrStackLocked = errors.New("stack is currently locked by another running operation")
)

type Manager struct {
	db          *database.DB
	agentClient *agent.Client
	stackLocks  sync.Map // stackID -> *sync.Mutex
	runningJobs sync.Map // jobID -> context.CancelFunc
}

func NewManager(db *database.DB, agentClient *agent.Client) *Manager {
	return &Manager{
		db:          db,
		agentClient: agentClient,
	}
}

func (m *Manager) getStackLock(stackID string) *sync.Mutex {
	actual, _ := m.stackLocks.LoadOrStore(stackID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func (m *Manager) SubmitComposeJob(
	ctx context.Context,
	jobType string,
	stack *models.Stack,
	removeVolumes bool,
	userEmail string,
) (*models.Job, error) {
	// Try acquiring stack lock without blocking HTTP request
	lock := m.getStackLock(stack.ID)
	if !lock.TryLock() {
		return nil, ErrStackLocked
	}

	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	job := &models.Job{
		ID:        jobID,
		Type:      jobType,
		StackID:   stack.ID,
		Status:    models.JobStatusRunning,
		StartedAt: time.Now(),
		CreatedBy: userEmail,
	}

	if err := m.db.CreateJob(job); err != nil {
		lock.Unlock()
		return nil, fmt.Errorf("failed to register job: %w", err)
	}

	jobCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	m.runningJobs.Store(jobID, cancel)

	go func() {
		defer func() {
			lock.Unlock()
			m.runningJobs.Delete(jobID)
			cancel()
		}()

		res, err := m.agentClient.ComposeAction(jobCtx, agent.ComposeActionRequest{
			StackName:      stack.Name,
			StackPath:      stack.Path,
			Action:         jobType,
			ComposeContent: stack.ComposeContent,
			EnvContent:     stack.EnvContent,
			RemoveVolumes:  removeVolumes,
		})

		completed := time.Now()
		job.CompletedAt = &completed

		if err != nil {
			job.Status = models.JobStatusFailed
			job.Error = err.Error()
		} else if !res.Success {
			job.Status = models.JobStatusFailed
			job.Error = res.Error
			job.Logs = res.Logs
		} else {
			job.Status = models.JobStatusCompleted
			job.Logs = res.Logs
		}

		_ = m.db.UpdateJob(job)
	}()

	return job, nil
}
