# Guia de Desenvolvimento & Contribuição 🛠️

Este documento descreve como desenvolvedores podem estender o CroMedia, adicionar novos codecs e filtros, e executar as suítes de testes e benchmarks da indústria.

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

Dispomos de três níveis de validação e testes: testes de unidade/integração, benchmarks de regressão de borda ("Casos Infernais") e projetos de arquitetura de alta escala.

### 1. Rodar os Testes Unitários e de Integração
Todos os testes Go nativos devem passar sem erros. Use build tags para incluir codecs legados:

```bash
# Executar testes unitários básicos do core
go test -v ./core/...

# Executar testes com suporte a codecs legados (AVI, ASF, RealMedia)
go test -tags "legacy legacy_avi legacy_asf legacy_rm legacy_mp2 legacy_codecs" -v ./...
```

### 2. Executar a Suíte de 100 "Casos Infernais" (Hellcases)
Avalia se decodificadores nativos, filtros e pipelines concorrentes resistem a lixos digitais corrompidos de rede e incompatibilidades:

```bash
go run benchmark/run_hellcases.go
```
Os relatórios detalhados de velocidade e memória contra o FFmpeg são gerados em:
* [report_hellcases.md](file:///home/j/Documentos/GitHub/cromedia/benchmark/report_hellcases.md)
* [expert_analysis_hellcases.md](file:///home/j/Documentos/GitHub/cromedia/benchmark/expert_analysis_hellcases.md)

### 3. Executar o Benchmark de Sincronismo (PTS-DTS Matrix & Sync)
Estressa o motor com desvios cronológicos extremos (FATE, OBS lags, RTSP drift, WebRTC jitter, TV Digital):

```bash
go run benchmark/run_sync_benchmarks.go
```
Gera logs estruturados e um dashboard visual em `benchmark/report_sync.html`.

---

## 🚀 Executando os Projetos de Arquitetura Práticos
No diretório `projetos_testes/`, criamos 6 simuladores práticos comparativos contra o FFmpeg:

1. **`cromedia-teste-panoptico`**: Ingestão de 100 streams RTSP alinhados por `SyncBarrier` para modelo YOLO (Edge NVR).
2. **`cromedia-teste-flash-transcoder`**: Transcoder serverless scale-to-zero com filtros de cor, loudness e watermark.
3. **`cromedia-teste-twitch-abr`**: Transcodificação ABR Ladder (1 input 1080p60 para 4 qualidades) concorrente de 20 streams.
4. **`cromedia-teste-cloudflare-ttfb`**: Extração de vídeo sob demanda JIT medindo o Time to First Byte (TTFB).
5. **`cromedia-teste-netflix-concat`**: Concatenação e alinhamento cronológico de 1.000 pequenos segmentos paralelos.
6. **`cromedia-teste-msu-codec`**: FPS vs VMAF qualidade perceptual sob bitrate limitado.

Para rodar qualquer um deles, entre na pasta do projeto e execute os subprogramas:

```bash
# 1. Executar a versão CroMedia
cd projetos_testes/cromedia-teste-panoptico/cromedia
go run main.go

# 2. Executar a versão FFmpeg
cd ../ffmpeg
go run main.go

# 3. Analisar os resultados JSON gravados na pasta resultados
cat ../resultados/panoptico_cromedia.json
cat ../resultados/panoptico_ffmpeg.json
```
A tabela geral comparativa dos 16 projetos práticos e análise crítica está localizada em [relatorio_comparativo.md](file:///home/j/Documentos/GitHub/cromedia/projetos_testes/relatorio_comparativo.md).

---

## 🏗️ Compilação CGO e Configuração de Desenvolvimento

Caso esteja desenvolvendo os wrappers CGO de produção (`cgo_x264.go` e `cgo_fdkaac.go`), você deve utilizar a tag de compilação `cgo_media`.

### Configuração de Dependências
Instale as bibliotecas de desenvolvimento no sistema anfitrião:
```bash
sudo apt-get update
sudo apt-get install -y libx264-dev libfdk-aac-dev
```

### Comandos de Compilação & Testes com CGO
```bash
# Executar todos os testes incluindo os wrappers CGO
go test -tags cgo_media -v ./core/...

# Compilar o CLI completo com aceleração CGO e debug de performance
go build -tags cgo_media -ldflags="-s -w" -o cromedia main.go
```

