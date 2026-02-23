package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending Status = "PENDING"
	StatusSuccess Status = "SUCCESS"
	StatusFailed  Status = "FAILED"
)

type Task struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Params    string     `json:"params,omitempty"`
	Status    Status     `json:"status"`
	Payload   string     `json:"payload,omitempty"` // 截图存文件路径，其他任务存原始 payload
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

type Store struct {
	mu    sync.RWMutex
	tasks []*Task
	file  string
}

func New() *Store {
	s := &Store{
		tasks: make([]*Task, 0),
		file:  "tasks.json",
	}
	s.load()
	return s
}

// Add 将新任务加入队列
func (s *Store) Add(taskType, params string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := &Task{
		ID:        uuid.NewString(),
		Type:      taskType,
		Params:    params,
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
	s.tasks = append(s.tasks, t)
	s.save()
	return t
}

// Pending 返回所有待执行任务。
// MAA 自身会按 ID 去重，所以可以重复返回相同任务直到它汇报完成。
func (s *Store) Pending() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Task
	for _, t := range s.tasks {
		if t.Status == StatusPending {
			result = append(result, t)
		}
	}
	return result
}

// Complete 标记任务完成
func (s *Store) Complete(id, status, payload string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tasks {
		if t.ID == id {
			t.Status = Status(status)
			t.Payload = payload
			now := time.Now()
			t.DoneAt = &now
			s.save()
			return true
		}
	}
	return false
}

// Get 按 ID 查找任务
func (s *Store) Get(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// All 返回所有任务（最新的在前）
func (s *Store) All() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, len(s.tasks))
	for i, t := range s.tasks {
		result[len(s.tasks)-1-i] = t
	}
	return result
}

func (s *Store) save() {
	data, _ := json.MarshalIndent(s.tasks, "", "  ")
	_ = os.WriteFile(s.file, data, 0644)
}

func (s *Store) load() {
	data, err := os.ReadFile(s.file)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &s.tasks)
}
