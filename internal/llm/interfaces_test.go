package llm

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing interfaces

// MockProvider - Mock implementation of Provider interface
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) CreateClient(config *Config) (Client, error) {
	args := m.Called(config)
	return args.Get(0).(Client), args.Error(1)
}

func (m *MockProvider) ValidateConfig(config *Config) error {
	args := m.Called(config)
	return args.Error(0)
}

// MockClient - Mock implementation of Client interface
type MockClient struct {
	mock.Mock
}

func (m *MockClient) Chat(ctx context.Context, req *ChatRequest, sessionID string) (*ChatResponse, error) {
	args := m.Called(ctx, req, sessionID)
	return args.Get(0).(*ChatResponse), args.Error(1)
}

func (m *MockClient) ChatStream(ctx context.Context, req *ChatRequest, sessionID string) (<-chan StreamDelta, error) {
	args := m.Called(ctx, req, sessionID)
	return args.Get(0).(<-chan StreamDelta), args.Error(1)
}

func (m *MockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockHTTPClient - Mock implementation of HTTPClient interface
type MockHTTPClient struct {
	MockClient
	httpClient *http.Client
}

func (m *MockHTTPClient) SetHTTPClient(client *http.Client) {
	m.httpClient = client
}

func (m *MockHTTPClient) GetHTTPClient() *http.Client {
	return m.httpClient
}

// MockStreamingClient - Mock implementation of StreamingClient interface
type MockStreamingClient struct {
	MockClient
	streamingEnabled bool
}

func (m *MockStreamingClient) SupportsStreaming() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockStreamingClient) SetStreamingEnabled(enabled bool) {
	m.streamingEnabled = enabled
}

// MockMetricsCollector - Mock implementation of MetricsCollector interface
type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) RecordRequest(model string, tokens int) {
	m.Called(model, tokens)
}

func (m *MockMetricsCollector) RecordResponse(model string, tokens int, duration int64) {
	m.Called(model, tokens, duration)
}

func (m *MockMetricsCollector) RecordError(model string, errorType string) {
	m.Called(model, errorType)
}

// MockClientFactory - Mock implementation of ClientFactory interface
type MockClientFactory struct {
	mock.Mock
}

func (m *MockClientFactory) CreateHTTPClient(config *Config) (HTTPClient, error) {
	args := m.Called(config)
	return args.Get(0).(HTTPClient), args.Error(1)
}

func (m *MockClientFactory) CreateStreamingClient(config *Config) (StreamingClient, error) {
	args := m.Called(config)
	return args.Get(0).(StreamingClient), args.Error(1)
}

func (m *MockClientFactory) GetSupportedProviders() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// Tests for interface implementations

func TestProviderInterface(t *testing.T) {
	mockProvider := new(MockProvider)

	// Test that mock implements the interface
	var _ Provider = mockProvider

	// Test Name method
	mockProvider.On("Name").Return("test-provider")
	name := mockProvider.Name()
	assert.Equal(t, "test-provider", name)

	// Test ValidateConfig method
	config := &Config{Model: "test-model"}
	mockProvider.On("ValidateConfig", config).Return(nil)
	err := mockProvider.ValidateConfig(config)
	assert.NoError(t, err)

	mockProvider.AssertExpectations(t)
}

func TestClientInterface(t *testing.T) {
	mockClient := new(MockClient)

	// Test that mock implements the interface
	var _ Client = mockClient

	// Test Chat method
	ctx := context.Background()
	req := &ChatRequest{Messages: []Message{{Role: "user", Content: "test"}}}
	expectedResponse := &ChatResponse{
		ID:    "test-response",
		Model: "test-model",
	}

	mockClient.On("Chat", ctx, req, "session123").Return(expectedResponse, nil)

	response, err := mockClient.Chat(ctx, req, "session123")

	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	// Test Close method
	mockClient.On("Close").Return(nil)
	err = mockClient.Close()
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestHTTPClientInterface(t *testing.T) {
	mockHTTPClient := new(MockHTTPClient)

	// Test that mock implements the interface
	var _ HTTPClient = mockHTTPClient
	var _ Client = mockHTTPClient // HTTPClient extends Client

	// Test SetHTTPClient and GetHTTPClient
	testClient := &http.Client{}
	mockHTTPClient.SetHTTPClient(testClient)

	retrievedClient := mockHTTPClient.GetHTTPClient()
	assert.Equal(t, testClient, retrievedClient)
}

func TestStreamingClientInterface(t *testing.T) {
	mockStreamingClient := new(MockStreamingClient)

	// Test that mock implements the interface
	var _ StreamingClient = mockStreamingClient
	var _ Client = mockStreamingClient // StreamingClient extends Client

	// Test SupportsStreaming method
	mockStreamingClient.On("SupportsStreaming").Return(true)
	supports := mockStreamingClient.SupportsStreaming()
	assert.True(t, supports)

	// Test SetStreamingEnabled method
	mockStreamingClient.SetStreamingEnabled(true)
	assert.True(t, mockStreamingClient.streamingEnabled)

	mockStreamingClient.SetStreamingEnabled(false)
	assert.False(t, mockStreamingClient.streamingEnabled)

	mockStreamingClient.AssertExpectations(t)
}

func TestMetricsCollectorInterface(t *testing.T) {
	mockCollector := new(MockMetricsCollector)

	// Test that mock implements the interface
	var _ MetricsCollector = mockCollector

	// Test RecordRequest method
	mockCollector.On("RecordRequest", "test-model", 100).Return()
	mockCollector.RecordRequest("test-model", 100)

	// Test RecordResponse method
	mockCollector.On("RecordResponse", "test-model", 150, int64(500)).Return()
	mockCollector.RecordResponse("test-model", 150, 500)

	// Test RecordError method
	mockCollector.On("RecordError", "test-model", "timeout").Return()
	mockCollector.RecordError("test-model", "timeout")

	mockCollector.AssertExpectations(t)
}

func TestClientFactoryInterface(t *testing.T) {
	mockFactory := new(MockClientFactory)

	// Test that mock implements the interface
	var _ ClientFactory = mockFactory

	// Test CreateHTTPClient method
	config := &Config{Model: "test-model"}
	mockHTTPClient := new(MockHTTPClient)
	mockFactory.On("CreateHTTPClient", config).Return(mockHTTPClient, nil)

	httpClient, err := mockFactory.CreateHTTPClient(config)
	assert.NoError(t, err)
	assert.Equal(t, mockHTTPClient, httpClient)

	// Test CreateStreamingClient method
	mockStreamingClient := new(MockStreamingClient)
	mockFactory.On("CreateStreamingClient", config).Return(mockStreamingClient, nil)

	streamingClient, err := mockFactory.CreateStreamingClient(config)
	assert.NoError(t, err)
	assert.Equal(t, mockStreamingClient, streamingClient)

	// Test GetSupportedProviders method
	expectedProviders := []string{"openai", "gemini", "ollama"}
	mockFactory.On("GetSupportedProviders").Return(expectedProviders)

	providers := mockFactory.GetSupportedProviders()
	assert.Equal(t, expectedProviders, providers)

	mockFactory.AssertExpectations(t)
}

func TestProviderCreateClient(t *testing.T) {
	mockProvider := new(MockProvider)
	mockClient := new(MockClient)

	config := &Config{
		Model:   "test-model",
		APIKey:  "test-key",
		BaseURL: "https://api.example.com",
	}

	mockProvider.On("CreateClient", config).Return(mockClient, nil)

	client, err := mockProvider.CreateClient(config)

	assert.NoError(t, err)
	assert.Equal(t, mockClient, client)
	mockProvider.AssertExpectations(t)
}

func TestClientChatStream(t *testing.T) {
	mockClient := new(MockClient)

	ctx := context.Background()
	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "test streaming"}},
		Stream:   true,
	}

	// Create a test channel
	streamChan := make(chan StreamDelta, 1)
	streamChan <- StreamDelta{
		ID:     "stream-1",
		Object: "chat.completion.chunk",
		Choices: []Choice{{
			Delta: Message{Content: "test response"},
		}},
	}
	close(streamChan)

	mockClient.On("ChatStream", ctx, req, "session123").Return((<-chan StreamDelta)(streamChan), nil)

	responseChan, err := mockClient.ChatStream(ctx, req, "session123")

	assert.NoError(t, err)
	assert.NotNil(t, responseChan)

	// Read from the channel
	delta := <-responseChan
	assert.Equal(t, "stream-1", delta.ID)
	assert.Equal(t, "test response", delta.Choices[0].Delta.Content)

	mockClient.AssertExpectations(t)
}