package knowledgegraph

import (
	"context"
	"fmt"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type OpenAIConfig struct {
	BaseURL        string
	APIKey         string
	EmbeddingModel string
}

type OpenAIEmbedder struct {
	client         openai.Client
	embeddingModel string
}

func NewOpenAIEmbedder(cfg OpenAIConfig) *OpenAIEmbedder {
	var opts []option.RequestOption
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	return &OpenAIEmbedder{
		client:         openai.NewClient(opts...),
		embeddingModel: cfg.EmbeddingModel,
	}
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e == nil {
		return nil, fmt.Errorf("knowledgegraph: openai embedder is nil")
	}
	if e.embeddingModel == "" {
		return nil, fmt.Errorf("knowledgegraph: embedding model required")
	}
	res, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.embeddingModel),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
	})
	if err != nil {
		return nil, err
	}
	if len(res.Data) == 0 {
		return nil, fmt.Errorf("knowledgegraph: empty embedding response")
	}
	out := make([]float32, len(res.Data[0].Embedding))
	for i := range res.Data[0].Embedding {
		out[i] = float32(res.Data[0].Embedding[i])
	}
	return out, nil
}
