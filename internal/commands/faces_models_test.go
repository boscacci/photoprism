package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/photoprism/photoprism/internal/ai/face"
	"github.com/photoprism/photoprism/internal/commands/catalog"
	"github.com/photoprism/photoprism/internal/photoprism"
)

func TestFacesModelsCommandJSON(t *testing.T) {
	output, err := RunWithTestContext(FacesModelsCommand, []string{"models", "--json"})

	if err != nil {
		t.Fatal(err)
	}

	var payload photoprism.FaceRecognitionModelList
	if err = json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, output)
	}

	assert.Equal(t, face.RecognitionModelFacenet, payload.ActiveModel)
	if assert.NotEmpty(t, payload.Models) {
		assert.Equal(t, face.RecognitionModelFacenet, payload.Models[0].Profile.Key)
	}
}

func TestFacesModelCommandsCatalog(t *testing.T) {
	commands := catalog.BuildFlat(FacesCommands, 0, "photoprism", false, nil)
	found := make(map[string]catalog.Command, len(commands))

	for _, command := range commands {
		found[command.FullName] = command
	}

	for _, want := range []string{
		"photoprism faces models",
		"photoprism faces recognize",
		"photoprism faces compare",
		"photoprism faces promote",
	} {
		if _, ok := found[want]; !ok {
			t.Fatalf("expected command %q in catalog", want)
		}
	}

	for _, command := range []catalog.Command{found["photoprism faces recognize"], found["photoprism faces compare"], found["photoprism faces promote"]} {
		hasModelFlag := false
		for _, flag := range command.Flags {
			if flag.Name == "--model" && flag.Required {
				hasModelFlag = true
			}
		}
		assert.Truef(t, hasModelFlag, "expected %s to require --model", command.FullName)
	}
}

func TestFaceRecognitionModelRows(t *testing.T) {
	rows, cols := faceRecognitionModelRows([]face.RecognitionAvailability{
		{
			Profile:   face.RecognitionProfile{Key: face.RecognitionModelAuraFaceV1, Name: "AuraFace v1", Class: "community"},
			Installed: true,
			Available: false,
			Reasons:   []string{face.RecognitionReasonMissingGPU},
			VRAMMiB:   4096,
		},
	})

	assert.Equal(t, []string{"Model", "Name", "Class", "Active", "Installed", "Available", "VRAM", "Reasons"}, cols)
	assert.Equal(t, face.RecognitionModelAuraFaceV1, rows[0][0])
	assert.Equal(t, "yes", rows[0][4])
	assert.Equal(t, "no", rows[0][5])
	assert.Equal(t, face.RecognitionReasonMissingGPU, rows[0][7])
}
