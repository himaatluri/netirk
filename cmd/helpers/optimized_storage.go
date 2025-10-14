package helpers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// OptimizedStorage implements DataStorage with performance optimizations for long-running sessions
type OptimizedStorage struct {
	session         *MonitoringSession
	mutex           sync.RWMutex
	performanceManager *PerformanceManager
	config          OptimizedStorageConfig
	dataBuffer      []MonitoringData
	bufferMutex     sync.RWMutex
	lastFlush       time.Time
	logger          *log.Logger
}

// OptimizedStorageConfig holds configuration for optimized storage
type OptimizedStorageConfig struct {
	MaxBufferSize     int           // Maximum number of data points in memory
	FlushInterval     time.Duration // How often to flush data to disk
	RotationSize      int           // Number of data points before rotation
	EnableCompression bool          // Enable data compression
	BackupEnabled     bool          // Enable automatic backups
}

// DefaultOptimizedStorageConfig returns sensible defaults for optimized storage
func DefaultOptimizedStorageConfig() OptimizedStorageConfig {
	return OptimizedStorageConfig{
		MaxBufferSize:     10000,
		FlushInterval:     5 * time.Minute,
		RotationSize:      50000,
		EnableCompression: false, // Disabled by default for simplicity
		BackupEnabled:     true,
	}
}

// NewOptimizedStorage creates a new optimized storage instance
func NewOptimizedStorage(config MonitoringConfig, perfManager *PerformanceManager, logger *log.Logger) *OptimizedStorage {
	if logger == nil {
		logger = log.Default()
	}

	storage := &OptimizedStorage{
		session: &MonitoringSession{
			StartTime: time.Now(),
			Config:    config,
			Data:      make([]MonitoringData, 0),
		},
		performanceManager: perfManager,
		config:            DefaultOptimizedStorageConfig(),
		dataBuffer:        make([]MonitoringData, 0),
		lastFlush:         time.Now(),
		logger:            logger,
	}

	// Start background flush routine
	go storage.backgroundFlushWorker()

	return storage
}

// Store adds a monitoring data point with memory management
func (s *OptimizedStorage) Store(data *MonitoringData) error {
	if data == nil {
		return CreateStorageError("store", fmt.Errorf("monitoring data cannot be nil"))
	}

	// Check memory constraints before storing
	if s.performanceManager != nil {
		withinLimit, memoryUsage := s.performanceManager.CheckMemoryUsage()
		if !withinLimit {
			s.logger.Printf("Memory usage high (%.2f MB), triggering data flush", memoryUsage)
			if err := s.flushBufferToSession(); err != nil {
				s.logger.Printf("Warning: failed to flush buffer during high memory usage: %v", err)
			}
		}
	}

	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	// Add to buffer
	s.dataBuffer = append(s.dataBuffer, *data)

	// Check if buffer needs flushing
	if len(s.dataBuffer) >= s.config.MaxBufferSize {
		s.logger.Printf("Buffer size limit reached (%d), flushing to session", s.config.MaxBufferSize)
		if err := s.flushBufferToSessionUnsafe(); err != nil {
			return CreateStorageError("buffer_flush", fmt.Errorf("failed to flush buffer: %w", err))
		}
	}

	return nil
}

// GetSession returns the current monitoring session with buffered data
func (s *OptimizedStorage) GetSession() *MonitoringSession {
	// Ensure buffer is flushed before returning session
	if err := s.flushBufferToSession(); err != nil {
		s.logger.Printf("Warning: failed to flush buffer before getting session: %v", err)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Create a copy to avoid race conditions
	sessionCopy := MonitoringSession{
		StartTime: s.session.StartTime,
		EndTime:   s.session.EndTime,
		Config:    s.session.Config,
		Data:      make([]MonitoringData, len(s.session.Data)),
	}
	copy(sessionCopy.Data, s.session.Data)

	return &sessionCopy
}

// SaveToFile saves the monitoring session to a JSON file with optimization
func (s *OptimizedStorage) SaveToFile(filename string) error {
	// Flush buffer before saving
	if err := s.flushBufferToSession(); err != nil {
		return CreateStorageError("pre_save_flush", fmt.Errorf("failed to flush buffer before save: %w", err))
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if filename == "" {
		return CreateStorageError("save", fmt.Errorf("filename cannot be empty"))
	}

	// Set end time when saving
	s.session.EndTime = time.Now()

	// Create backup if enabled
	if s.config.BackupEnabled {
		backupFilename := fmt.Sprintf("%s.backup-%s", filename, time.Now().Format("20060102-150405"))
		if err := s.saveToFileUnsafe(backupFilename); err != nil {
			s.logger.Printf("Warning: failed to create backup: %v", err)
		}
	}

	return s.saveToFileUnsafe(filename)
}

// saveToFileUnsafe performs the actual file save operation (must be called with mutex held)
func (s *OptimizedStorage) saveToFileUnsafe(filename string) error {
	data, err := json.MarshalIndent(s.session, "", "  ")
	if err != nil {
		return CreateStorageError("marshal", fmt.Errorf("failed to marshal monitoring data: %w", err))
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return CreateStorageError("write", fmt.Errorf("failed to write monitoring data to file %s: %w", filename, err))
	}

	return nil
}

// LoadFromFile loads a monitoring session from a JSON file
func (s *OptimizedStorage) LoadFromFile(filename string) error {
	if filename == "" {
		return CreateStorageError("load", fmt.Errorf("filename cannot be empty"))
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return CreateStorageError("load", fmt.Errorf("monitoring data file does not exist: %s", filename))
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return CreateStorageError("read", fmt.Errorf("failed to read monitoring data file %s: %w", filename, err))
	}

	var session MonitoringSession
	err = json.Unmarshal(data, &session)
	if err != nil {
		return CreateStorageError("unmarshal", fmt.Errorf("failed to unmarshal monitoring data from %s: %w", filename, err))
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.session = &session
	return nil
}

// GetLatestData returns the most recent monitoring data point for a target
func (s *OptimizedStorage) GetLatestData(target string) *MonitoringData {
	// Check buffer first for most recent data
	s.bufferMutex.RLock()
	for i := len(s.dataBuffer) - 1; i >= 0; i-- {
		if s.dataBuffer[i].Target == target {
			dataCopy := s.dataBuffer[i]
			s.bufferMutex.RUnlock()
			return &dataCopy
		}
	}
	s.bufferMutex.RUnlock()

	// Check session data
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for i := len(s.session.Data) - 1; i >= 0; i-- {
		if s.session.Data[i].Target == target {
			dataCopy := s.session.Data[i]
			return &dataCopy
		}
	}

	return nil
}

// SetEndTime sets the end time for the monitoring session
func (s *OptimizedStorage) SetEndTime(endTime time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.session.EndTime = endTime
}

// GetBufferSize returns the current size of the data buffer
func (s *OptimizedStorage) GetBufferSize() int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()
	
	bufferSize := len(s.dataBuffer)
	
	s.mutex.RLock()
	sessionSize := len(s.session.Data)
	s.mutex.RUnlock()
	
	return bufferSize + sessionSize
}

// FlushToFile saves current data to file and optionally clears the buffer
func (s *OptimizedStorage) FlushToFile(filename string, clearBuffer bool) error {
	if err := s.SaveToFile(filename); err != nil {
		return err
	}

	if clearBuffer {
		s.clearBufferAndSession()
	}

	return nil
}

// flushBufferToSession moves data from buffer to session (thread-safe)
func (s *OptimizedStorage) flushBufferToSession() error {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()
	return s.flushBufferToSessionUnsafe()
}

// flushBufferToSessionUnsafe moves data from buffer to session (must be called with buffer mutex held)
func (s *OptimizedStorage) flushBufferToSessionUnsafe() error {
	if len(s.dataBuffer) == 0 {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check for rotation if session is getting too large
	if len(s.session.Data)+len(s.dataBuffer) > s.config.RotationSize {
		if err := s.rotateDataUnsafe(); err != nil {
			s.logger.Printf("Warning: failed to rotate data: %v", err)
		}
	}

	// Append buffer data to session
	s.session.Data = append(s.session.Data, s.dataBuffer...)
	
	// Clear buffer
	s.dataBuffer = s.dataBuffer[:0]
	s.lastFlush = time.Now()

	return nil
}

// rotateDataUnsafe performs data rotation to manage memory (must be called with mutex held)
func (s *OptimizedStorage) rotateDataUnsafe() error {
	if len(s.session.Data) == 0 {
		return nil
	}

	// Save current data to rotation file
	rotationFilename := fmt.Sprintf("monitoring-rotation-%s.json", time.Now().Format("20060102-150405"))
	
	// Create a temporary session for rotation
	rotationSession := MonitoringSession{
		StartTime: s.session.StartTime,
		EndTime:   time.Now(),
		Config:    s.session.Config,
		Data:      s.session.Data,
	}

	data, err := json.MarshalIndent(rotationSession, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rotation data: %w", err)
	}

	err = os.WriteFile(rotationFilename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write rotation file: %w", err)
	}

	s.logger.Printf("Rotated %d data points to %s", len(s.session.Data), rotationFilename)

	// Keep only recent data (last 25% of rotation size)
	keepSize := s.config.RotationSize / 4
	if len(s.session.Data) > keepSize {
		s.session.Data = s.session.Data[len(s.session.Data)-keepSize:]
	}

	// Force garbage collection after rotation
	if s.performanceManager != nil {
		s.performanceManager.ForceGarbageCollection()
	}

	return nil
}

// clearBufferAndSession clears both buffer and session data
func (s *OptimizedStorage) clearBufferAndSession() {
	s.bufferMutex.Lock()
	s.dataBuffer = s.dataBuffer[:0]
	s.bufferMutex.Unlock()

	s.mutex.Lock()
	s.session.Data = s.session.Data[:0]
	s.session.StartTime = time.Now()
	s.session.EndTime = time.Time{}
	s.mutex.Unlock()
}

// backgroundFlushWorker runs periodic buffer flushes
func (s *OptimizedStorage) backgroundFlushWorker() {
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for range ticker.C {
		if time.Since(s.lastFlush) >= s.config.FlushInterval {
			s.bufferMutex.RLock()
			bufferSize := len(s.dataBuffer)
			s.bufferMutex.RUnlock()

			if bufferSize > 0 {
				s.logger.Printf("Background flush: moving %d data points from buffer to session", bufferSize)
				if err := s.flushBufferToSession(); err != nil {
					s.logger.Printf("Warning: background flush failed: %v", err)
				}
			}
		}
	}
}

// GetStorageStats returns storage performance statistics
func (s *OptimizedStorage) GetStorageStats() map[string]interface{} {
	s.bufferMutex.RLock()
	bufferSize := len(s.dataBuffer)
	s.bufferMutex.RUnlock()

	s.mutex.RLock()
	sessionSize := len(s.session.Data)
	s.mutex.RUnlock()

	return map[string]interface{}{
		"buffer_size":        bufferSize,
		"session_size":       sessionSize,
		"total_size":         bufferSize + sessionSize,
		"max_buffer_size":    s.config.MaxBufferSize,
		"rotation_size":      s.config.RotationSize,
		"last_flush":         s.lastFlush.Format("15:04:05"),
		"flush_interval":     s.config.FlushInterval.String(),
		"backup_enabled":     s.config.BackupEnabled,
	}
}

// ClearData clears all stored data and resets the session
func (s *OptimizedStorage) ClearData() {
	s.clearBufferAndSession()
}

// GetDataForTarget returns all monitoring data for a specific target
func (s *OptimizedStorage) GetDataForTarget(target string) []MonitoringData {
	// Flush buffer to ensure we get all data
	s.flushBufferToSession()
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var result []MonitoringData
	for _, data := range s.session.Data {
		if data.Target == target {
			result = append(result, data)
		}
	}
	
	return result
}

// GetDataSince returns all monitoring data since the specified time
func (s *OptimizedStorage) GetDataSince(since time.Time) []MonitoringData {
	// Flush buffer to ensure we get all data
	s.flushBufferToSession()
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var result []MonitoringData
	for _, data := range s.session.Data {
		if data.Timestamp.After(since) {
			result = append(result, data)
		}
	}
	
	return result
}