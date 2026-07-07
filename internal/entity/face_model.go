package entity

import (
	"crypto/sha1" //nolint:gosec // G505: Stable non-cryptographic face identifier hash.
	"encoding/base32"
	"encoding/json"
	"fmt"
	"time"

	"github.com/photoprism/photoprism/internal/ai/face"
)

// FaceModelEmbedding stores candidate model embeddings for an existing face marker.
type FaceModelEmbedding struct {
	ModelKey       string          `gorm:"type:VARBINARY(64);primary_key;auto_increment:false;" json:"ModelKey" yaml:"ModelKey"`
	MarkerUID      string          `gorm:"type:VARBINARY(42);primary_key;auto_increment:false;" json:"MarkerUID" yaml:"MarkerUID"`
	EmbeddingsJSON json.RawMessage `gorm:"type:MEDIUMBLOB;" json:"-" yaml:"EmbeddingsJSON,omitempty"`
	embeddings     face.Embeddings `gorm:"-" yaml:"-"`
	CreatedAt      time.Time       `json:"CreatedAt" yaml:"CreatedAt,omitempty"`
	UpdatedAt      time.Time       `json:"UpdatedAt" yaml:"UpdatedAt,omitempty"`
}

// TableName returns the entity table name.
func (FaceModelEmbedding) TableName() string {
	return "face_model_embeddings"
}

// NewFaceModelEmbedding returns a candidate embedding row.
func NewFaceModelEmbedding(modelKey, markerUID string, embeddings face.Embeddings) (*FaceModelEmbedding, error) {
	result := &FaceModelEmbedding{
		ModelKey:  face.ParseRecognitionModel(modelKey),
		MarkerUID: markerUID,
	}

	return result, result.SetEmbeddings(embeddings)
}

// SetEmbeddings assigns candidate marker embeddings.
func (m *FaceModelEmbedding) SetEmbeddings(embeddings face.Embeddings) error {
	if m == nil {
		return fmt.Errorf("face model embedding is nil")
	}

	if embeddings.Empty() {
		return fmt.Errorf("invalid embedding")
	}

	m.embeddings = embeddings
	m.EmbeddingsJSON = embeddings.JSON()

	return nil
}

// Embeddings returns parsed candidate marker embeddings.
func (m *FaceModelEmbedding) Embeddings() face.Embeddings {
	if m == nil || len(m.EmbeddingsJSON) == 0 {
		return face.Embeddings{}
	} else if len(m.embeddings) > 0 {
		return m.embeddings
	} else if err := json.Unmarshal(m.EmbeddingsJSON, &m.embeddings); err != nil {
		log.Errorf("faces: failed parsing candidate embedding json: %s", err)
	}

	return m.embeddings
}

// FaceModelCluster stores candidate model face clusters isolated from active FaceNet state.
type FaceModelCluster struct {
	ID              string          `gorm:"type:VARBINARY(64);primary_key;auto_increment:false;" json:"ID" yaml:"ID"`
	ModelKey        string          `gorm:"type:VARBINARY(64);index;default:'';" json:"ModelKey" yaml:"ModelKey"`
	FaceSrc         string          `gorm:"type:VARBINARY(8);" json:"Src" yaml:"Src,omitempty"`
	FaceKind        int             `json:"Kind" yaml:"Kind,omitempty"`
	FaceHidden      bool            `json:"Hidden" yaml:"Hidden,omitempty"`
	SubjUID         string          `gorm:"type:VARBINARY(42);index;default:'';" json:"SubjUID" yaml:"SubjUID,omitempty"`
	Samples         int             `json:"Samples" yaml:"Samples,omitempty"`
	SampleRadius    float64         `json:"SampleRadius" yaml:"SampleRadius,omitempty"`
	Collisions      int             `json:"Collisions" yaml:"Collisions,omitempty"`
	CollisionRadius float64         `json:"CollisionRadius" yaml:"CollisionRadius,omitempty"`
	EmbeddingJSON   json.RawMessage `gorm:"type:MEDIUMBLOB;" json:"-" yaml:"EmbeddingJSON,omitempty"`
	embedding       face.Embedding  `gorm:"-" yaml:"-"`
	MatchedAt       *time.Time      `json:"MatchedAt" yaml:"MatchedAt,omitempty"`
	CreatedAt       time.Time       `json:"CreatedAt" yaml:"CreatedAt,omitempty"`
	UpdatedAt       time.Time       `json:"UpdatedAt" yaml:"UpdatedAt,omitempty"`
}

// TableName returns the entity table name.
func (FaceModelCluster) TableName() string {
	return "face_model_clusters"
}

// NewFaceModelCluster returns a new candidate face cluster.
func NewFaceModelCluster(modelKey, subjUID, faceSrc string, embeddings face.Embeddings) (*FaceModelCluster, error) {
	result := &FaceModelCluster{
		ModelKey: face.ParseRecognitionModel(modelKey),
		SubjUID:  subjUID,
		FaceSrc:  faceSrc,
	}

	return result, result.SetEmbeddings(embeddings)
}

// SetEmbeddings assigns cluster embeddings and derives a model-scoped id.
func (m *FaceModelCluster) SetEmbeddings(embeddings face.Embeddings) error {
	if m == nil {
		return fmt.Errorf("face model cluster is nil")
	}

	if len(embeddings) == 0 {
		return fmt.Errorf("invalid embedding")
	}

	m.embedding, m.SampleRadius, m.Samples = face.EmbeddingsMidpoint(embeddings)

	if len(m.embedding) != len(face.NullEmbedding) {
		return fmt.Errorf("embedding has invalid number of values")
	}

	if m.SampleRadius > face.ClusterRadius {
		m.SampleRadius = face.ClusterRadius
	}

	var err error
	m.EmbeddingJSON, err = json.Marshal(m.embedding)

	if err != nil {
		return err
	}

	m.ID = faceModelClusterID(m.ModelKey, m.EmbeddingJSON)

	if k := int(m.embedding.Kind()); k > m.FaceKind {
		m.FaceKind = k
	}

	m.MatchedAt = nil

	return nil
}

// Embedding returns the parsed cluster embedding.
func (m *FaceModelCluster) Embedding() face.Embedding {
	if m == nil || len(m.EmbeddingJSON) == 0 {
		return face.Embedding{}
	} else if len(m.embedding) > 0 {
		return m.embedding
	} else if err := json.Unmarshal(m.EmbeddingJSON, &m.embedding); err != nil {
		log.Errorf("faces: failed parsing candidate cluster embedding json: %s", err)
	}

	return m.embedding
}

// ToFace converts a candidate cluster to an active face entity.
func (m *FaceModelCluster) ToFace() (*Face, error) {
	if m == nil {
		return nil, fmt.Errorf("face model cluster is nil")
	}

	f := NewFace(m.SubjUID, m.FaceSrc, face.Embeddings{m.Embedding()})
	if f == nil {
		return nil, fmt.Errorf("face is nil")
	}

	f.FaceHidden = m.FaceHidden
	f.FaceKind = m.FaceKind
	f.Collisions = m.Collisions
	f.CollisionRadius = m.CollisionRadius

	return f, nil
}

// FaceModelMarker stores candidate model marker matches isolated from active FaceNet state.
type FaceModelMarker struct {
	ModelKey  string     `gorm:"type:VARBINARY(64);primary_key;auto_increment:false;" json:"ModelKey" yaml:"ModelKey"`
	MarkerUID string     `gorm:"type:VARBINARY(42);primary_key;auto_increment:false;" json:"MarkerUID" yaml:"MarkerUID"`
	FaceID    string     `gorm:"type:VARBINARY(64);index;" json:"FaceID" yaml:"FaceID,omitempty"`
	FaceDist  float64    `gorm:"default:-1;" json:"FaceDist" yaml:"FaceDist,omitempty"`
	SubjUID   string     `gorm:"type:VARBINARY(42);index;default:'';" json:"SubjUID" yaml:"SubjUID,omitempty"`
	SubjSrc   string     `gorm:"type:VARBINARY(8);default:'';" json:"SubjSrc" yaml:"SubjSrc,omitempty"`
	MatchedAt *time.Time `sql:"index" json:"MatchedAt" yaml:"MatchedAt,omitempty"`
	CreatedAt time.Time  `json:"CreatedAt" yaml:"CreatedAt,omitempty"`
	UpdatedAt time.Time  `json:"UpdatedAt" yaml:"UpdatedAt,omitempty"`
}

// TableName returns the entity table name.
func (FaceModelMarker) TableName() string {
	return "face_model_markers"
}

// FaceModelRun stores candidate preparation state for a recognition model.
type FaceModelRun struct {
	ModelKey       string     `gorm:"type:VARBINARY(64);primary_key;auto_increment:false;" json:"ModelKey" yaml:"ModelKey"`
	Status         string     `gorm:"type:VARBINARY(32);default:'';" json:"Status" yaml:"Status,omitempty"`
	MarkerCount    int        `json:"MarkerCount" yaml:"MarkerCount,omitempty"`
	EmbeddingCount int        `json:"EmbeddingCount" yaml:"EmbeddingCount,omitempty"`
	ClusterCount   int        `json:"ClusterCount" yaml:"ClusterCount,omitempty"`
	MatchedCount   int        `json:"MatchedCount" yaml:"MatchedCount,omitempty"`
	Error          string     `gorm:"type:VARCHAR(255);default:'';" json:"Error" yaml:"Error,omitempty"`
	PreparedAt     *time.Time `json:"PreparedAt" yaml:"PreparedAt,omitempty"`
	PromotedAt     *time.Time `json:"PromotedAt" yaml:"PromotedAt,omitempty"`
	CreatedAt      time.Time  `json:"CreatedAt" yaml:"CreatedAt,omitempty"`
	UpdatedAt      time.Time  `json:"UpdatedAt" yaml:"UpdatedAt,omitempty"`
}

// TableName returns the entity table name.
func (FaceModelRun) TableName() string {
	return "face_model_runs"
}

// faceModelClusterID returns a stable model-scoped candidate cluster id.
func faceModelClusterID(modelKey string, embedding json.RawMessage) string {
	//nolint:gosec // G401: Stable identifier hash; not used for security decisions.
	s := sha1.Sum(append([]byte(modelKey+"\x00"), embedding...))
	return base32.StdEncoding.EncodeToString(s[:])
}
