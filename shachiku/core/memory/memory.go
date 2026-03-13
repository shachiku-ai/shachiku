package memory

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/philippgille/chromem-go"
	"gorm.io/gorm"

	"shachiku/core/config"
	"shachiku/core/models"
)

var (
	sqlDB   *gorm.DB
	chroDB  *chromem.DB
	col     *chromem.Collection
	colName = "agent_memory"
)

// Init sets up the embedded chromem-go database and SQLite
func Init() {
	var err error

	// Create data directories
	if err := os.MkdirAll(filepath.Join(config.GetDataDir(), "tmp"), 0755); err != nil {
		log.Printf("[Memory] Failed to create tmp directory: %v", err)
	}

	// Initialize SQLite for Short-term memory history
	sqlDB, err = gorm.Open(sqlite.Open(filepath.Join(config.GetDataDir(), "chat_history.db")), &gorm.Config{})
	if err != nil {
		log.Printf("[Memory] SQLite init failed: %v", err)
	} else {
		err = sqlDB.AutoMigrate(&models.Message{}, &models.LLMConfig{}, &models.Task{}, &models.TaskLog{}, &models.TokenLog{})
		if err != nil {
			log.Printf("[Memory] SQLite migration failed: %v", err)
		} else {
			log.Println("[Memory] SQLite chat memory initialized.")
		}
	}

	// Initialize the embedded chromem-go database with local file persistence for Long-term memory.
	// This creates a `.chromem` folder in the data directory.
	chroDB, err = chromem.NewPersistentDB(filepath.Join(config.GetDataDir(), ".chromem"), false)
	if err != nil {
		log.Printf("[Memory] Local Vector DB initialization failed: %v", err)
		return
	}

	// Try to get or create the collection
	col = chroDB.GetCollection(colName, nil)
	if col == nil {
		col, err = chroDB.CreateCollection(colName, nil, nil)
		if err != nil {
			log.Printf("[Memory] Failed to create vector collection: %v", err)
			return
		}
		log.Printf("[Memory] Created local collection '%s'", colName)
	} else {
		log.Printf("[Memory] Successfully loaded local collection '%s'", colName)
	}

	log.Println("[Memory] Long-term memory initialized (Chromem).")
}

// AddMessage appends a new message to the SQLite short-term memory
func AddMessage(role, content string) {
	if sqlDB != nil {
		sqlDB.Create(&models.Message{Role: role, Content: content, CreatedAt: time.Now()})
	}
}

// GetRecentHistory returns short-term memory from SQLite (last 100 messages in chronological order)
func GetRecentHistory() []models.Message {
	var messages []models.Message
	if sqlDB != nil {
		// Fetch the latest 100 messages
		sqlDB.Order("created_at desc").Limit(100).Find(&messages)

		// Reverse the slice to maintain chronological order (oldest first)
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	return messages
}

// ClearShortTermMemory deletes all messages from SQLite
func ClearShortTermMemory() {
	if sqlDB != nil {
		sqlDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Message{})
	}
}

// SaveFactToLongTermMemory stores an embedded fact locally using Chromem
func SaveFactToLongTermMemory(text string, embedding []float32) error {
	if chroDB == nil || col == nil {
		return fmt.Errorf("local memory Vector DB not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pointId := uuid.New().String()

	err := col.AddDocument(ctx, chromem.Document{
		ID:        pointId,
		Content:   text,
		Embedding: embedding,
		Metadata: map[string]string{
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		},
	})

	if err == nil {
		log.Printf("[Memory] Fact saved to local DB: %s...", text[:min(len(text), 30)])
	}
	return err
}

// SearchMemory retrieves semantically similar facts from the local Chromem DB
func SearchMemory(queryEmbedding []float32, limit uint64) ([]string, error) {
	if chroDB == nil || col == nil {
		return nil, fmt.Errorf("local memory Vector DB not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := col.QueryEmbedding(ctx, queryEmbedding, int(limit), nil, nil)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, doc := range res {
		results = append(results, doc.Content)
	}
	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DeleteFactFromLongTermMemory removes an item from Chromem
func DeleteFactFromLongTermMemory(id string) error {
	if chroDB == nil || col == nil {
		return fmt.Errorf("local memory Vector DB not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return col.Delete(ctx, nil, nil, id)
}

// GetAllLongTermMemory retrieves a list of all stored facts
func GetAllLongTermMemory() ([]models.Fact, error) {
	if chroDB == nil || col == nil {
		return nil, fmt.Errorf("local memory Vector DB not initialized")
	}

	count := col.Count()
	if count == 0 {
		return []models.Fact{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Provide a dummy embedding to fetch everything (size up to 'count')
	dummy := make([]float32, 1536)
	dummy[0] = 1.0

	res, err := col.QueryEmbedding(ctx, dummy, count, nil, nil)
	if err != nil {
		return nil, err
	}

	var results []models.Fact
	for _, doc := range res {
		results = append(results, models.Fact{
			ID:        doc.ID,
			Content:   doc.Content,
			Timestamp: doc.Metadata["timestamp"],
		})
	}
	return results, nil
}

// GetLLMConfig returns the LLM config from SQLite or defaults
func GetLLMConfig() models.LLMConfig {
	var cfg models.LLMConfig
	if sqlDB != nil {
		if err := sqlDB.First(&cfg, 1).Error; err != nil {
			cfg = models.LLMConfig{ID: 1, Provider: "openai", MaxIterations: 50}
			sqlDB.Create(&cfg)
		} else if cfg.MaxIterations == 0 {
			cfg.MaxIterations = 50
			sqlDB.Save(&cfg)
		}
	} else {
		cfg.Provider = "openai"
		cfg.MaxIterations = 50
	}
	return cfg
}

// UpdateLLMConfig saves the LLM config back to SQLite
func UpdateLLMConfig(newCfg models.LLMConfig) error {
	if sqlDB == nil {
		return fmt.Errorf("local sqlDB not initialized")
	}
	newCfg.ID = 1 // Ensure we only have one configuration row
	return sqlDB.Save(&newCfg).Error
}

// CreateTask saves a dynamically created task to the database
func CreateTask(name, cron, prompt string) (*models.Task, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("local sqlDB not initialized")
	}
	task := &models.Task{
		Name:      name,
		Cron:      cron,
		Prompt:    prompt,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	err := sqlDB.Create(task).Error
	return task, err
}

// GetTasks retrieves all tasks from the database
func GetTasks() []models.Task {
	var tasks []models.Task
	if sqlDB != nil {
		sqlDB.Order("created_at asc").Find(&tasks)
	}
	return tasks
}

// GetTaskLogs retrieves all logs for a specific task
func GetTaskLogs(taskID uint) []models.TaskLog {
	var logs []models.TaskLog
	if sqlDB != nil {
		sqlDB.Where("task_id = ?", taskID).Order("created_at asc").Find(&logs)
	}
	return logs
}

// UpdateTaskStatus updates a task's status
func UpdateTaskStatus(id uint, status string) {
	if sqlDB != nil {
		sqlDB.Model(&models.Task{}).Where("id = ?", id).Update("status", status)
	}
}

// CreateTaskLog saves a log entry for a specific executed task
func CreateTaskLog(taskID uint, output string) {
	if sqlDB != nil {
		sqlDB.Create(&models.TaskLog{
			TaskID:    taskID,
			Output:    output,
			CreatedAt: time.Now(),
		})
	}
}

// LogTokenUsage records token usage into the DB
func LogTokenUsage(taskID uint, inputTokens, outputTokens int) {
	if sqlDB != nil {
		sqlDB.Create(&models.TokenLog{
			TaskID:       taskID,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			CreatedAt:    time.Now(),
		})
	}
}

type DailyTokenUsage struct {
	Date         string `json:"date"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

type TaskTokenUsage struct {
	TaskID       uint   `json:"task_id"`
	TaskName     string `json:"task_name"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

type TokenDashboardMetrics struct {
	DailyUsage []DailyTokenUsage `json:"daily_usage"`
	TaskUsage  []TaskTokenUsage  `json:"task_usage"`
}

// GetTokenDashboardMetrics retrieves token statistics for the dashboard
func GetTokenDashboardMetrics() (TokenDashboardMetrics, error) {
	var metrics TokenDashboardMetrics

	if sqlDB == nil {
		return metrics, fmt.Errorf("local sqlDB not initialized")
	}

	// Initialize 30 days of data with zeros
	now := time.Now()
	var allDays []DailyTokenUsage
	dayMap := make(map[string]int)

	for i := 29; i >= 0; i-- {
		dayStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		allDays = append(allDays, DailyTokenUsage{
			Date:         dayStr,
			InputTokens:  0,
			OutputTokens: 0,
		})
		dayMap[dayStr] = len(allDays) - 1
	}

	thirtyDaysAgo := now.AddDate(0, 0, -30)

	var dbResults []DailyTokenUsage
	// Daily usage for the last 30 days
	// SQLite syntax for grouping by date: strftime('%Y-%m-%d', created_at)
	err := sqlDB.Model(&models.TokenLog{}).
		Select("strftime('%Y-%m-%d', created_at) as date, sum(input_tokens) as input_tokens, sum(output_tokens) as output_tokens").
		Where("created_at >= ?", thirtyDaysAgo).
		Group("strftime('%Y-%m-%d', created_at)").
		Order("date ASC").
		Scan(&dbResults).Error

	if err != nil {
		return metrics, fmt.Errorf("failed to fetch daily token usage: %v", err)
	}

	for _, result := range dbResults {
		if idx, ok := dayMap[result.Date]; ok {
			allDays[idx].InputTokens = result.InputTokens
			allDays[idx].OutputTokens = result.OutputTokens
		}
	}

	metrics.DailyUsage = allDays

	// Task usage for all time
	err = sqlDB.Table("token_logs").
		Select("token_logs.task_id, tasks.name as task_name, sum(token_logs.input_tokens) as input_tokens, sum(token_logs.output_tokens) as output_tokens").
		Joins("LEFT JOIN tasks ON tasks.id = token_logs.task_id").
		Group("token_logs.task_id, tasks.name").
		Order("sum(token_logs.input_tokens) + sum(token_logs.output_tokens) DESC").
		Scan(&metrics.TaskUsage).Error

	if err != nil {
		return metrics, fmt.Errorf("failed to fetch task token usage: %v", err)
	}

	return metrics, nil
}

// DeleteTasksByName finds tasks by name, deletes them from SQLite, and returns them so we can unschedule them.
func DeleteTasksByName(name string) ([]models.Task, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("local sqlDB not initialized")
	}
	var tasks []models.Task
	if err := sqlDB.Where("name = ?", name).Find(&tasks).Error; err != nil || len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks found with name '%s'", name)
	}

	for _, t := range tasks {
		sqlDB.Delete(&t)
		sqlDB.Where("task_id = ?", t.ID).Delete(&models.TaskLog{})
	}

	return tasks, nil
}

// DeleteTaskByID removes a specific task and its logs by ID
func DeleteTaskByID(id uint) error {
	if sqlDB == nil {
		return fmt.Errorf("local sqlDB not initialized")
	}

	// Delete task logs first
	sqlDB.Where("task_id = ?", id).Delete(&models.TaskLog{})

	// Delete the task itself
	res := sqlDB.Where("id = ?", id).Delete(&models.Task{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("task not found")
	}

	return nil
}

// ClearTasks deletes all tasks and task logs from SQLite
func ClearTasks() {
	if sqlDB != nil {
		sqlDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.TaskLog{})
		sqlDB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Task{})
	}
}
