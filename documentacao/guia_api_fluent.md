# Guia da API Fluente (Fluent API) para Go 🧩

Para desenvolvedores que queiram integrar o CroMedia diretamente como biblioteca em seus projetos Go, o framework provê um construtor de pipelines fluente (`PipelineBuilder`). Esta API abstrai a complexidade de instanciar canais, demuxers e workers concorrentes.

---

## 🏗️ Como Importar

Importe o pacote core no seu arquivo `.go`:

```go
import "cromedia/core"
```

---

## 🛠️ Exemplo 1: Processamento Básico de Vídeo e Áudio

O builder permite encadear passos como redimensionamento, ajuste de ganho de áudio, definição de entradas/saídas e execução ordenada.

```go
package main

import (
	"fmt"
	"cromedia/core"
)

func main() {
	// Definir caminhos de entrada e saída
	inputFile := "entrada.mp4"
	outputFile := "saida.mp4"

	// Construir e executar o pipeline
	err := core.Input(inputFile).
		Scale(1280, 720).    // Redimensionar para HD 720p
		Volume(1.5).         // Ganho de 150% no volume do áudio
		Output(outputFile).  // Definir destino
		Run()                // Executar concorrentemente

	if err != nil {
		fmt.Printf("Falha ao rodar pipeline: %v\n", err)
		return
	}

	fmt.Println("Processamento de mídia concluído com sucesso!")
}
```

---

## ⚡ Exemplo 2: Transcodificador Avançado com Múltiplos Filtros (Estilo Serverless/UGC)

Para fluxos complexos como injeção de marca d'água visual, normalização adaptativa e correção de cor (como o projeto **Flash-Transcoder**):

```go
package main

import (
	"context"
	"fmt"
	"cromedia/core"
)

func main() {
	pctx := core.NewPipelineContext(context.Background())
	
	// Carrega uma marca d'água RGBA na memória
	watermarkFrame := getWatermarkFrame() 

	// Executa pipeline completo com encadeamento de filtros
	err := core.Input("ugc_input.mp4").
		Scale(1280, 720).
		ColorGrade(10.0, 1.15). // Brightness +10, Contrast 1.15
		Watermark(watermarkFrame, 1140, 660). // Marca d'água no canto inferior direito
		DrawText("© CroMedia", 20, 20). // Texto estático no topo
		LoudnessNormalize(-1.0). // Normalização preditiva de loudness (single-pass) a -1dB
		Output("processed_segment.ts").
		RunWithContext(pctx) // Executa coletando telemetria e gerenciamento de pânicos

	if err != nil {
		fmt.Printf("Erro de transcodificação: %v\n", err)
		return
	}

	// Imprimir relatório de performance
	pctx.PrintReport()
}

func getWatermarkFrame() *core.VideoFrame {
	// Retorna VideoFrame RGBA
	return &core.VideoFrame{Width: 100, Height: 30, Format: core.PixelFormatRGBA, Data: make([]byte, 100*30*4)}
}
```

---

## ⚙️ Funcionamento Interno da Fluent API

Ao invocar `.Run()` ou `.RunWithContext()`, o builder executa as seguintes etapas estruturadas:
1. **Sniffing de Cabeçalho**: Lê magic bytes do input para abrir o demuxer correspondente.
2. **BufferPool Allocation**: Aloca frames de vídeo e chunks de áudio utilizando o pool global para elidir alocações desnecessárias.
3. **Filtros Concorrentes**: Instancia as structs de filtros (`ScaleFilter`, `ColorFilter`, `OverlayFilter`, `DrawTextFilter` e `PredictiveGainNormalizer`) processando scanlines concorrentemente.
4. **Timestamp Ordering**: Acopla o `reorderMap` e `SyncBarrier` para garantir que o áudio normalizado e o vídeo processado permaneçam perfeitamente sincronizados no muxer de saída.
5. **Auto-Cleanup**: Libera os buffers e fecha os descritores no término do pipeline, evitando vazamentos (memory leaks) e deadlocks de rede.
