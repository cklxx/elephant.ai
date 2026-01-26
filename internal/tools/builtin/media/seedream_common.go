package media

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"alex/internal/httpclient"
	"alex/internal/jsonx"
	"alex/internal/logging"
	"alex/internal/utils"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

// SeedreamConfig captures common configuration for the Seedream tools.
type SeedreamConfig struct {
	APIKey          string
	Model           string
	ModelDescriptor string
	ModelEnvVar     string
}

const (
	VisionProviderSeedream = "seedream"
)

// VisionConfig allows selecting a vision provider; seedream is the default.
type VisionConfig struct {
	Provider string
	Seedream SeedreamConfig
}

type seedreamClient interface {
	GenerateImages(ctx context.Context, request arkm.GenerateImagesRequest) (arkm.ImagesResponse, error)
	CreateResponses(ctx context.Context, request *responses.ResponsesRequest) (*responses.ResponseObject, error)
	CreateContentGenerationTask(ctx context.Context, request arkm.CreateContentGenerationTaskRequest) (*arkm.CreateContentGenerationTaskResponse, error)
	GetContentGenerationTask(ctx context.Context, request arkm.GetContentGenerationTaskRequest) (*arkm.GetContentGenerationTaskResponse, error)
}

type seedreamAPIClient struct {
	client *arkruntime.Client
}

func (c *seedreamAPIClient) GenerateImages(ctx context.Context, request arkm.GenerateImagesRequest) (arkm.ImagesResponse, error) {
	return c.client.GenerateImages(ctx, request)
}

func (c *seedreamAPIClient) CreateResponses(ctx context.Context, request *responses.ResponsesRequest) (*responses.ResponseObject, error) {
	return c.client.CreateResponses(ctx, request)
}

func (c *seedreamAPIClient) CreateContentGenerationTask(ctx context.Context, request arkm.CreateContentGenerationTaskRequest) (*arkm.CreateContentGenerationTaskResponse, error) {
	resp, err := c.client.CreateContentGenerationTask(ctx, request)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *seedreamAPIClient) GetContentGenerationTask(ctx context.Context, request arkm.GetContentGenerationTaskRequest) (*arkm.GetContentGenerationTaskResponse, error) {
	resp, err := c.client.GetContentGenerationTask(ctx, request)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

type seedreamClientFactory struct {
	config SeedreamConfig
	once   sync.Once
	client seedreamClient
	err    error
}

func (f *seedreamClientFactory) instance() (seedreamClient, error) {
	f.once.Do(func() {
		if f.client != nil {
			return
		}
		apiKey := strings.TrimSpace(f.config.APIKey)
		if apiKey == "" {
			f.err = errors.New("seedream API key missing")
			return
		}
		httpLogger := logging.NewComponentLogger("SeedreamHTTP")
		f.client = &seedreamAPIClient{client: arkruntime.NewClientWithApiKey(apiKey, arkruntime.WithHTTPClient(httpclient.New(10*time.Minute, httpLogger)))}
	})
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

const (
	// doubao-seedance-1.0-pro documentation: https://www.volcengine.com/docs/82379/1587798
	seedanceMinDurationSeconds = 2
	seedanceMaxDurationSeconds = 12

	seedreamMaxInlineVideoBytes  = 40 * 1024 * 1024
	seedreamMaxInlineImageBytes  = 8 * 1024 * 1024
	seedreamMaxInlineBinaryBytes = 4 * 1024 * 1024
	seedreamAssetHTTPTimeout     = 2 * time.Minute

	seedreamMinGuidanceScale     = 1.0
	seedreamMaxGuidanceScale     = 10.0
	seedreamDefaultGuidanceScale = 7.0
	seedreamDefaultImageSize     = "1920x1920"
	seedreamMinImagePixels       = 3686400
)

var seedreamPlaceholderNonce = func() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func logSeedreamRequestPayload(requestID string, payload any) {
	if encoded, err := jsonx.Marshal(payload); err == nil {
		utils.LogStreamingRequestPayload(strings.TrimSpace(requestID), encoded)
	}
}

func logSeedreamResponsePayload(requestID string, payload any) {
	if encoded, err := jsonx.Marshal(payload); err == nil {
		utils.LogStreamingResponsePayload(strings.TrimSpace(requestID), encoded)
	}
}
