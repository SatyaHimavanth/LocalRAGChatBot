// internal/engine/engine.go
package engine

import (
    llama "github.com/tcpipuk/llama-go"
)

type Engine struct {
    ChatModel  *llama.Model
    EmbedModel *llama.Model
    embedCtx   *llama.Context // embeddings are stateless per call — one shared context is enough
}

func New(chatPath, embedPath string) (*Engine, error) {
    chatModel, err := llama.LoadModel(chatPath, llama.WithGPULayers(-1))
    if err != nil {
        return nil, err
    }
    embedModel, err := llama.LoadModel(embedPath, llama.WithGPULayers(-1))
    if err != nil {
        return nil, err
    }
    embedCtx, err := embedModel.NewContext(llama.WithEmbeddings())
    if err != nil {
        return nil, err
    }
    return &Engine{ChatModel: chatModel, EmbedModel: embedModel, embedCtx: embedCtx}, nil
}

func (e *Engine) Embed(text string) ([]float32, error) {
    return e.embedCtx.GetEmbeddings(text)
}

func (e *Engine) Close() {
    e.embedCtx.Close()
    e.EmbedModel.Close()
    e.ChatModel.Close()
}