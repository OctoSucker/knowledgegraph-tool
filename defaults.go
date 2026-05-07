package knowledgegraph

import "os"

const (
	DefaultIngestModel      = "gpt-4o-mini"
	DefaultIngestConfidence = 0.65
)

func IngestModelFromEnv() string {
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		return v
	}
	return DefaultIngestModel
}
