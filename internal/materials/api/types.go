package api

import "time"

// MaterialStatus mirrors the proto enum but is defined manually to avoid
// requiring generated code before the control plane exists.
type MaterialStatus int32

const (
	MaterialStatusUnspecified  MaterialStatus = 0
	MaterialStatusInput        MaterialStatus = 1
	MaterialStatusIntermediate MaterialStatus = 2
	MaterialStatusFinal        MaterialStatus = 3
)

// Visibility defines how broadly a material may be accessed.
type Visibility int32

const (
	VisibilityUnspecified Visibility = 0
	VisibilityPrivate     Visibility = 1
	VisibilityShared      Visibility = 2
	VisibilityPublic      Visibility = 3
)

// RequestContext binds a material to a particular runtime invocation.
type RequestContext struct {
	RequestID      string
	TaskID         string
	AgentIteration uint32
	ToolCallID     string
	ConversationID string
	UserID         string
}

// MaterialInput represents a single material to register.
type MaterialInput struct {
	Name                string
	MimeType            string
	InlineBytes         []byte
	ExternalURI         string
	StorageKey          string
	CDNURL              string
	ContentHash         string
	SizeBytes           uint64
	Description         string
	Source              string
	Status              MaterialStatus
	Origin              string
	Tags                map[string]string
	Annotations         map[string]string
	Visibility          Visibility
	Lineage             *LineageEdgeInput
	SystemAttributes    *SystemAttributes
	AccessBindings      []*AccessBinding
	RetentionTTLSeconds uint64
}

// LineageEdgeInput describes the parent edge for a newly created material.
type LineageEdgeInput struct {
	ParentMaterialID string
	DerivationType   string
	ParametersHash   string
}

// RegisterMaterialsRequest is sent to the registry when new outputs are
// available.
type RegisterMaterialsRequest struct {
	Context   *RequestContext
	Materials []*MaterialInput
}

// RegisterMaterialsResponse returns the newly created catalog entries.
type RegisterMaterialsResponse struct {
	Materials []*Material
}

// Material aggregates descriptor, storage, lineage, and ACL state.
type Material struct {
	MaterialID       string
	Descriptor       *MaterialDescriptor
	Storage          *MaterialStorage
	Context          *RequestContext
	Lineage          []*LineageEdge
	AccessBindings   []*AccessBinding
	SystemAttributes *SystemAttributes
}

// MaterialDescriptor provides human-facing metadata for rendering.
type MaterialDescriptor struct {
	Name                string
	Placeholder         string
	MimeType            string
	Description         string
	Source              string
	Origin              string
	Status              string
	Visibility          Visibility
	Tags                map[string]string
	Annotations         map[string]string
	RetentionTTLSeconds uint64
}

// MaterialStorage captures persistent storage information.
type MaterialStorage struct {
	StorageKey  string
	CDNURL      string
	ContentHash string
	SizeBytes   uint64
}

// LineageEdge connects derived materials.
type LineageEdge struct {
	ParentMaterialID string
	ChildMaterialID  string
	DerivationType   string
	ParametersHash   string
}

// AccessBinding records ACL grants.
type AccessBinding struct {
	Principal  string
	Scope      string
	Capability string
	ExpiresAt  time.Time
}

// SystemAttributes stores runtime indexed metadata used for governance/search.
type SystemAttributes struct {
	DomainTags     []string
	ComplianceTags []string
	EmbeddingsRef  string
	VectorIndexKey string
	Extra          map[string]string
}

// ListMaterialsRequest describes a snapshot request.
type ListMaterialsRequest struct {
	RequestID     string
	UpToIteration uint32
	Statuses      []MaterialStatus
}

// ListMaterialsResponse is returned by ListMaterials.
type ListMaterialsResponse struct {
	Materials []*Material
}

// MaterialEvent is emitted over the watch stream when a material changes.
type MaterialEvent struct {
	RequestID   string
	Material    *Material
	TombstoneID string
}
