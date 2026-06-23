# Guia de Desenvolvimento & Contribuição 🛠️

Este documento descreve como desenvolvedores podem estender o CroMedia, adicionando novos codecs, novos filtros e executando a suíte de testes e benchmarks.

---

## 🎨 Como Adicionar um Novo Filtro de Vídeo ou Áudio

Todos os filtros herdam interfaces simples que operam em buffers brutos de pixel ou som.

### 1. Criar Filtro de Vídeo Personalizado
Crie uma struct que implemente a interface `VideoFilter` localizada em `core/video_filter.go`:

```go
type MeuFiltroDeVideo struct {
	Parametro int
}

func (f *MeuFiltroDeVideo) Process(frame *VideoFrame) (*VideoFrame, error) {
	if frame.Format != PixelFormatRGBA {
		return frame, nil // ignora se não for RGBA por simplicidade
	}
	
	// Cria uma cópia do frame ou processa inline
	outData := make([]byte, len(frame.Data))
	copy(outData, frame.Data)
	
	// Exemplo: escurece os pixels
	for i := 0; i < len(outData); i += 4 {
		outData[i] = byte(int(outData[i]) / 2)   // R
		outData[i+1] = byte(int(outData[i+1]) / 2) // G
		outData[i+2] = byte(int(outData[i+2]) / 2) // B
	}
	
	return &VideoFrame{
		Width:  frame.Width,
		Height: frame.Height,
		Format: frame.Format,
		Data:   outData,
	}, nil
}
```

---

## 📹 Como Registrar um Novo Codec

Os codecs devem ser registrados na fábrica global em `core/codec.go` usando `RegisterCodec`.

```go
func init() {
	core.RegisterCodec(
		core.Codec{
			Name:        "meucodec",
			Type:        core.TrackTypeVideo,
			Description: "Meu Codec de Teste",
		},
		func() (interface{}, error) { return &MeuDecoder{}, nil },
		func() (interface{}, error) { return &MeuEncoder{}, nil },
	)
}
```

---

## 🧪 Rodando Testes e Benchmarks

### 1. Rodar os Testes Unitários e de Integração
Todos os testes Go nativos devem passar sem erros:

```bash
go test -v ./...
```

### 2. Executar a Suíte de 100 Testes de Benchmark
Mede a eficiência de tempo e de RAM em relação ao FFmpeg:

```bash
go run benchmark/run_benchmarks.go
```
Verifique os relatórios atualizados em `benchmark/report.md`.
