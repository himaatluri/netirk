package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// MonitoringData represents a single monitoring data point
type MonitoringData struct {
	Timestamp    time.Time     `json:"timestamp"`
	Target       string        `json:"target"`
	Status       string        `json:"status"`
	ResponseTime time.Duration `json:"response_time_ms"`
	StatusCode   int           `json:"status_code,omitempty"`
	Error        string        `json:"error,omitempty"`
	SSLInfo      *SSLData      `json:"ssl_info,omitempty"`
	TraceData    *NetworkTrace `json:"trace_data,omitempty"`
}

// NetworkTrace contains detailed network timing information
type NetworkTrace struct {
	DNSTime       time.Duration `json:"dns_time_ms"`
	ConnectTime   time.Duration `json:"connect_time_ms"`
	TLSTime       time.Duration `json:"tls_time_ms"`
	FirstByteTime time.Duration `json:"first_byte_time_ms"`
}

// SSLData contains SSL certificate information
type SSLData struct {
	ExpiryDate   time.Time `json:"expiry_date"`
	DaysToExpiry int       `json:"days_to_expiry"`
	Issuer       string    `json:"issuer"`
	IsValid      bool      `json:"is_valid"`
	Error        string    `json:"error,omitempty"`
}

// MonitoringSession represents a complete monitoring session
type MonitoringSession struct {
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time"`
	Config    MonitoringConfig `json:"config"`
	Data      []MonitoringData `json:"data"`
	mutex     sync.RWMutex     `json:"-"`
}

// MonitoringConfig holds configuration for a monitoring session
type MonitoringConfig struct {
	Targets  []string      `json:"targets"`
	Interval time.Duration `json:"interval"`
	Duration time.Duration `json:"duration,omitempty"`
}

// DataCollector interface defines methods for collecting monitoring data
type DataCollector interface {
	CollectData(target string) (*MonitoringData, error)
	SetTimeout(timeout time.Duration)
}

// DataStorage interface defines methods for storing and retrieving monitoring data
type DataStorage interface {
	Store(data *MonitoringData) error
	GetSession() *MonitoringSession
	SaveToFile(filename string) error
	LoadFromFile(filename string) error
	GetLatestData(target string) *MonitoringData
	SetEndTime(endTime time.Time)
	GetBufferSize() int
	FlushToFile(filename string, clearBuffer bool) error
}

// InMemoryStorage implements DataStorage interface with in-memory storage
type InMemoryStorage struct {
	session *MonitoringSession
	mutex   sync.RWMutex
}

// NewInMemoryStorage creates a new in-memory storage instance
func NewInMemoryStorage(config MonitoringConfig) *InMemoryStorage {
	return &InMemoryStorage{
		session: &MonitoringSession{
			StartTime: time.Now(),
			Config:    config,
			Data:      make([]MonitoringData, 0),
		},
	}
}

// Store adds a monitoring data point to the session with error handling
func (s *InMemoryStorage) Store(data *MonitoringData) error {
	if data == nil {
		return CreateStorageError("store", fmt.Errorf("monitoring data cannot be nil"))
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check for memory constraints (basic protection against memory leaks)
	if len(s.session.Data) > 100000 { // Limit to 100k data points
		return CreateStorageError("store", fmt.Errorf("storage limit exceeded: too many data points"))
	}

	s.session.Data = append(s.session.Data, *data)
	return nil
}

// GetSession returns the current monitoring session
func (s *InMemoryStorage) GetSession() *MonitoringSession {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Create a copy to avoid race conditions, excluding the mutex
	sessionCopy := MonitoringSession{
		StartTime: s.session.StartTime,
		EndTime:   s.session.EndTime,
		Config:    s.session.Config,
		Data:      make([]MonitoringData, len(s.session.Data)),
	}
	copy(sessionCopy.Data, s.session.Data)

	return &sessionCopy
}

// SaveToFile saves the monitoring session to a JSON file with error handling
func (s *InMemoryStorage) SaveToFile(filename string) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Validate filename
	if strings.TrimSpace(filename) == "" {
		return CreateStorageError("save", fmt.Errorf("filename cannot be empty"))
	}

	// Set end time when saving
	s.session.EndTime = time.Now()

	// Marshal data with error handling
	data, err := json.MarshalIndent(s.session, "", "  ")
	if err != nil {
		return CreateStorageError("marshal", fmt.Errorf("failed to marshal monitoring data: %w", err))
	}

	// Write file with error handling
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return CreateStorageError("write", fmt.Errorf("failed to write monitoring data to file %s: %w", filename, err))
	}

	return nil
}

// LoadFromFile loads a monitoring session from a JSON file with error handling
func (s *InMemoryStorage) LoadFromFile(filename string) error {
	// Validate filename
	if strings.TrimSpace(filename) == "" {
		return CreateStorageError("load", fmt.Errorf("filename cannot be empty"))
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return CreateStorageError("load", fmt.Errorf("monitoring data file does not exist: %s", filename))
	}

	// Read file with error handling
	data, err := os.ReadFile(filename)
	if err != nil {
		return CreateStorageError("read", fmt.Errorf("failed to read monitoring data file %s: %w", filename, err))
	}

	// Unmarshal data with error handling
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

// GetDataCount returns the number of monitoring data points collected
func (s *InMemoryStorage) GetDataCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.session.Data)
}

// GetLatestData returns the most recent monitoring data point for a target
func (s *InMemoryStorage) GetLatestData(target string) *MonitoringData {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Search backwards for the most recent data point for this target
	for i := len(s.session.Data) - 1; i >= 0; i-- {
		if s.session.Data[i].Target == target {
			dataCopy := s.session.Data[i]
			return &dataCopy
		}
	}

	return nil
}

// ClearData removes all monitoring data from the session
func (s *InMemoryStorage) ClearData() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.session.Data = make([]MonitoringData, 0)
	s.session.StartTime = time.Now()
	s.session.EndTime = time.Time{}
}

// GetDataForTarget returns all monitoring data points for a specific target
func (s *InMemoryStorage) GetDataForTarget(target string) []MonitoringData {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var targetData []MonitoringData
	for _, data := range s.session.Data {
		if data.Target == target {
			targetData = append(targetData, data)
		}
	}

	return targetData
}

// GetDataSince returns all monitoring data points since a specific time
func (s *InMemoryStorage) GetDataSince(since time.Time) []MonitoringData {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var recentData []MonitoringData
	for _, data := range s.session.Data {
		if data.Timestamp.After(since) {
			recentData = append(recentData, data)
		}
	}

	return recentData
}

// SetEndTime sets the end time for the monitoring session
func (s *InMemoryStorage) SetEndTime(endTime time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.session.EndTime = endTime
}

// GetBufferSize returns the current size of the data buffer
func (s *InMemoryStorage) GetBufferSize() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.session.Data)
}

// FlushToFile saves current data to file and optionally clears the buffer
func (s *InMemoryStorage) FlushToFile(filename string, clearBuffer bool) error {
	if err := s.SaveToFile(filename); err != nil {
		return err
	}

	if clearBuffer {
		s.ClearData()
	}

	return nil
}
