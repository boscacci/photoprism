package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize/english"
	"github.com/urfave/cli/v2"

	aiface "github.com/photoprism/photoprism/internal/ai/face"
	"github.com/photoprism/photoprism/internal/entity"
	"github.com/photoprism/photoprism/internal/photoprism"
	"github.com/photoprism/photoprism/pkg/txt/report"
)

// FacesModelsCommand displays configured face recognition model candidates.
var FacesModelsCommand = &cli.Command{
	Name:   "models",
	Usage:  "Lists face recognition models",
	Flags:  report.CliFlags,
	Action: facesModelsAction,
}

// FacesRecognizeModelCommand prepares candidate embeddings and clusters.
var FacesRecognizeModelCommand = &cli.Command{
	Name:  "recognize",
	Usage: "Prepares a face recognition model candidate",
	Flags: []cli.Flag{
		faceModelFlag(),
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "regenerate all candidate embeddings",
		},
		JsonFlag(),
	},
	Action: facesRecognizeModelAction,
}

// FacesCompareModelCommand compares a prepared candidate with the active model.
var FacesCompareModelCommand = &cli.Command{
	Name:   "compare",
	Usage:  "Compares a prepared face recognition model candidate",
	Flags:  []cli.Flag{faceModelFlag(), JsonFlag()},
	Action: facesCompareModelAction,
}

// FacesPromoteModelCommand promotes a prepared candidate as the active model.
var FacesPromoteModelCommand = &cli.Command{
	Name:   "promote",
	Usage:  "Promotes a prepared face recognition model candidate",
	Flags:  []cli.Flag{faceModelFlag(), JsonFlag()},
	Action: facesPromoteModelAction,
}

// faceModelFlag returns the shared model selector flag for face model commands.
func faceModelFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:     "model",
		Usage:    "face recognition model `KEY`",
		Required: true,
	}
}

// facesModelsAction lists face recognition model candidates.
func facesModelsAction(ctx *cli.Context) error {
	conf, err := InitCoreConfig(ctx, true)
	if err != nil {
		log.Debug(err)
	}

	format, formatErr := report.CliFormatStrict(ctx)
	if formatErr != nil {
		return formatErr
	}

	requestCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	models, err := photoprism.NewFaces(conf).RecognitionModels(requestCtx)
	if err != nil {
		return err
	}

	if format == report.JSON {
		return printJSON(models)
	}

	rows, cols := faceRecognitionModelRows(models.Models)
	result, err := report.RenderFormat(rows, cols, format)
	fmt.Println(result)
	return err
}

// facesRecognizeModelAction prepares candidate state for a recognition model.
func facesRecognizeModelAction(ctx *cli.Context) error {
	start := time.Now()

	conf, err := InitConfig(ctx)
	requestCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err != nil {
		return err
	}

	conf.InitDb()
	defer conf.Shutdown()

	modelKey := aiface.ParseRecognitionModel(ctx.String("model"))
	run, err := photoprism.NewFaces(conf).RecognizeModel(requestCtx, modelKey, ctx.Bool("force"))
	if err != nil {
		return err
	}

	if ctx.Bool("json") {
		return printJSON(run)
	}

	printFaceModelRun(run)
	log.Infof("prepared %s recognition model in %s", modelKey, time.Since(start))

	return nil
}

// facesCompareModelAction compares candidate and active model state.
func facesCompareModelAction(ctx *cli.Context) error {
	conf, err := InitConfig(ctx)
	if err != nil {
		return err
	}

	conf.InitDb()
	defer conf.Shutdown()

	modelKey := aiface.ParseRecognitionModel(ctx.String("model"))
	result, err := photoprism.NewFaces(conf).CompareRecognitionModel(modelKey)
	if err != nil {
		return err
	}

	if ctx.Bool("json") {
		return printJSON(result)
	}

	rows, cols := faceRecognitionCompareRows(result)
	table, err := report.RenderFormat(rows, cols, report.Default)
	fmt.Println(table)
	return err
}

// facesPromoteModelAction promotes a prepared candidate model.
func facesPromoteModelAction(ctx *cli.Context) error {
	start := time.Now()

	conf, err := InitConfig(ctx)
	if err != nil {
		return err
	}

	conf.InitDb()
	defer conf.Shutdown()

	modelKey := aiface.ParseRecognitionModel(ctx.String("model"))
	if err = photoprism.NewFaces(conf).PromoteRecognitionModel(modelKey); err != nil {
		return err
	}

	payload := map[string]string{
		"model":  modelKey,
		"status": photoprism.FaceModelStatusPromoted,
	}

	if ctx.Bool("json") {
		return printJSON(payload)
	}

	log.Infof("promoted %s recognition model in %s", modelKey, time.Since(start))
	fmt.Printf("Promoted %s recognition model.\n", modelKey)

	return nil
}

// faceRecognitionModelRows returns display rows for recognition model availability.
func faceRecognitionModelRows(models []aiface.RecognitionAvailability) ([][]string, []string) {
	rows := make([][]string, 0, len(models))

	for _, model := range models {
		rows = append(rows, []string{
			model.Profile.Key,
			model.Profile.Name,
			model.Profile.Class,
			yesNo(model.Active),
			yesNo(model.Installed),
			yesNo(model.Available),
			faceRecognitionVRAM(model.VRAMMiB),
			faceRecognitionReasons(model.Reasons),
		})
	}

	return rows, []string{"Model", "Name", "Class", "Active", "Installed", "Available", "VRAM", "Reasons"}
}

// faceRecognitionCompareRows returns display rows for candidate comparison metrics.
func faceRecognitionCompareRows(result photoprism.FaceRecognitionCompareResult) ([][]string, []string) {
	rows := [][]string{
		{"model", result.ModelKey},
		{"active model", result.ActiveModel},
		{"prepared", yesNo(result.Prepared)},
		{"active clusters", strconv.Itoa(result.ActiveClusters)},
		{"candidate clusters", strconv.Itoa(result.CandidateClusters)},
		{"active markers", strconv.Itoa(result.ActiveMarkers)},
		{"candidate markers", strconv.Itoa(result.CandidateMarkers)},
		{"active recognized", strconv.Itoa(result.ActiveRecognized)},
		{"candidate recognized", strconv.Itoa(result.CandidateRecognized)},
		{"changed subjects", strconv.Itoa(result.ChangedSubjects)},
		{"missing candidate rows", strconv.Itoa(result.MissingCandidateRows)},
	}

	return rows, []string{"Metric", "Value"}
}

// printFaceModelRun prints a compact candidate preparation summary.
func printFaceModelRun(run *entity.FaceModelRun) {
	if run == nil {
		return
	}

	rows := [][]string{
		{"model", run.ModelKey},
		{"status", run.Status},
		{"markers", strconv.Itoa(run.MarkerCount)},
		{"embeddings", strconv.Itoa(run.EmbeddingCount)},
		{"clusters", strconv.Itoa(run.ClusterCount)},
		{"matched markers", strconv.Itoa(run.MatchedCount)},
	}

	if strings.TrimSpace(run.Error) != "" {
		rows = append(rows, []string{"error", run.Error})
	}

	result, err := report.RenderFormat(rows, []string{"Metric", "Value"}, report.Default)
	if err == nil {
		fmt.Println(result)
	}
}

// faceRecognitionReasons returns a compact reason list for table output.
func faceRecognitionReasons(reasons []string) string {
	if len(reasons) == 0 {
		return "-"
	}

	return strings.Join(reasons, ", ")
}

// faceRecognitionVRAM formats VRAM metadata for table output.
func faceRecognitionVRAM(vramMiB int) string {
	if vramMiB <= 0 {
		return "-"
	}

	return english.Plural(vramMiB, "MiB", "MiB")
}

// yesNo returns a stable display value for booleans.
func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}
