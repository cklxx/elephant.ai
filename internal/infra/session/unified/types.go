package unified

import "time"

// Surface identifies a delivery channel.
type Surface string

const (
	SurfaceLark Surface = "lark"
	SurfaceCLI  Surface = "cli"
	SurfaceWeb  Surface = "web"
	SurfaceAPI  Surface = "api"
)

// SurfaceBinding links a surface-specific ID to a unified session.
type SurfaceBinding struct {
	Surface   Surface   `json:"surface"`
	SurfaceID string    `json:"surface_id"`
	SessionID string    `json:"session_id"`
	BoundAt   time.Time `json:"bound_at"`
}
