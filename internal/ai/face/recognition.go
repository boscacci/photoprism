package face

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// RecognitionModelFacenet identifies the legacy TensorFlow FaceNet model.
	RecognitionModelFacenet = "facenet"
	// RecognitionModelAuraFaceV1 identifies the AuraFace v1 ONNX model profile.
	RecognitionModelAuraFaceV1 = "auraface-v1"
	// RecognitionModelInsightFacePrefix identifies BYO InsightFace model profiles.
	RecognitionModelInsightFacePrefix = "insightface-"
)

const (
	// RecognitionReasonMissingModel indicates that model weights are not installed.
	RecognitionReasonMissingModel = "missing_model"
	// RecognitionReasonMissingGPU indicates that no CUDA-capable GPU is reachable.
	RecognitionReasonMissingGPU = "missing_gpu"
	// RecognitionReasonInsufficientVRAM indicates that the GPU does not meet VRAM requirements.
	RecognitionReasonInsufficientVRAM = "insufficient_vram"
	// RecognitionReasonMissingService indicates that no recognition service endpoint is configured.
	RecognitionReasonMissingService = "missing_service_uri"
	// RecognitionReasonSidecarUnhealthy indicates that the configured recognition service is unhealthy.
	RecognitionReasonSidecarUnhealthy = "sidecar_unhealthy"
	// RecognitionReasonIncompletePreparation indicates that a candidate model has not been fully prepared.
	RecognitionReasonIncompletePreparation = "incomplete_preparation"
)

// RecognitionProfile describes a face recognition embedding model.
type RecognitionProfile struct {
	Key             string   `json:"key"`
	Name            string   `json:"name"`
	Class           string   `json:"class"`
	License         string   `json:"license,omitempty"`
	InputSize       int      `json:"inputSize"`
	EmbeddingDims   int      `json:"embeddingDims"`
	MinVRAMMiB      int      `json:"minVramMiB,omitempty"`
	RequiresGPU     bool     `json:"requiresGpu"`
	RequiresService bool     `json:"requiresService"`
	RequiresWeights bool     `json:"requiresWeights"`
	Hidden          bool     `json:"hidden"`
	WeightFiles     []string `json:"-"`
}

// RecognitionProbe contains deployment facts used to decide model availability.
type RecognitionProbe struct {
	ModelsPath     string
	ConfiguredKey  string
	GPUName        string
	VRAMMiB        int
	CUDARuntime    bool
	ServiceURI     string
	ServiceChecked bool
	ServiceHealthy bool
}

// RecognitionAvailability reports whether a model can be used.
type RecognitionAvailability struct {
	Profile   RecognitionProfile `json:"profile"`
	Installed bool               `json:"installed"`
	Available bool               `json:"available"`
	Active    bool               `json:"active"`
	Reasons   []string           `json:"reasons,omitempty"`
	GPUName   string             `json:"gpuName,omitempty"`
	VRAMMiB   int                `json:"vramMiB,omitempty"`
}

var recognitionProfiles = []RecognitionProfile{
	{
		Key:           RecognitionModelFacenet,
		Name:          "FaceNet",
		Class:         "legacy",
		License:       "Apache-2.0",
		InputSize:     160,
		EmbeddingDims: 512,
	},
	{
		Key:             RecognitionModelAuraFaceV1,
		Name:            "AuraFace v1 (4x GPU)",
		Class:           "community",
		License:         "Apache-2.0",
		InputSize:       112,
		EmbeddingDims:   512,
		MinVRAMMiB:      8000,
		RequiresGPU:     true,
		RequiresService: true,
		RequiresWeights: true,
		WeightFiles:     []string{"model.onnx", "glintr100.onnx", "auraface-v1.onnx"},
	},
	{
		Key:             "insightface-buffalo-l",
		Name:            "InsightFace Buffalo-L",
		Class:           "byo",
		License:         "commercial/BYO",
		InputSize:       112,
		EmbeddingDims:   512,
		MinVRAMMiB:      12000,
		RequiresGPU:     true,
		RequiresService: true,
		RequiresWeights: true,
		Hidden:          true,
		WeightFiles:     []string{"model.onnx", "buffalo_l.onnx"},
	},
}

// RecognitionProfiles returns configured recognition profiles.
func RecognitionProfiles(includeHidden bool) []RecognitionProfile {
	result := make([]RecognitionProfile, 0, len(recognitionProfiles))

	for _, profile := range recognitionProfiles {
		if profile.Hidden && !includeHidden {
			continue
		}

		result = append(result, profile)
	}

	return result
}

// RecognitionProfileByKey returns a profile for the specified model key.
func RecognitionProfileByKey(key string) (RecognitionProfile, bool) {
	key = ParseRecognitionModel(key)

	for _, profile := range recognitionProfiles {
		if profile.Key == key {
			return profile, true
		}
	}

	return RecognitionProfile{}, false
}

// ParseRecognitionModel normalizes recognition model keys.
func ParseRecognitionModel(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))

	switch key {
	case "", "face-net", "tensorflow", "default":
		return RecognitionModelFacenet
	default:
		return key
	}
}

// WeightPaths returns possible model weight locations for this profile.
func (p RecognitionProfile) WeightPaths(modelsPath string) []string {
	if !p.RequiresWeights || len(p.WeightFiles) == 0 {
		return nil
	}

	base := filepath.Join(modelsPath, p.Key)
	result := make([]string, 0, len(p.WeightFiles))

	for _, name := range p.WeightFiles {
		if strings.TrimSpace(name) == "" {
			continue
		}

		result = append(result, filepath.Join(base, name))
	}

	return result
}

// Installed reports whether required model weights are present.
func (p RecognitionProfile) Installed(modelsPath string) bool {
	if !p.RequiresWeights {
		return true
	}

	for _, name := range p.WeightPaths(modelsPath) {
		if info, err := os.Stat(name); err == nil && !info.IsDir() && info.Size() > 0 {
			return true
		}
	}

	return false
}

// Availability evaluates whether this profile can be used in the probed deployment.
func (p RecognitionProfile) Availability(probe RecognitionProbe) RecognitionAvailability {
	result := RecognitionAvailability{
		Profile:   p,
		Installed: p.Installed(probe.ModelsPath),
		Active:    ParseRecognitionModel(probe.ConfiguredKey) == p.Key,
		GPUName:   probe.GPUName,
		VRAMMiB:   probe.VRAMMiB,
	}

	if !result.Installed {
		result.Reasons = append(result.Reasons, RecognitionReasonMissingModel)
	}

	if p.RequiresGPU {
		if !probe.CUDARuntime {
			result.Reasons = append(result.Reasons, RecognitionReasonMissingGPU)
		}

		if p.MinVRAMMiB > 0 && probe.VRAMMiB > 0 && probe.VRAMMiB < p.MinVRAMMiB {
			result.Reasons = append(result.Reasons, RecognitionReasonInsufficientVRAM)
		}
	}

	if p.RequiresService {
		if strings.TrimSpace(probe.ServiceURI) == "" {
			result.Reasons = append(result.Reasons, RecognitionReasonMissingService)
		} else if probe.ServiceChecked && !probe.ServiceHealthy {
			result.Reasons = append(result.Reasons, RecognitionReasonSidecarUnhealthy)
		}
	}

	result.Available = len(result.Reasons) == 0

	return result
}
