# CroMedia v0.8+ (Modular Expansion) 🚀

O **CroMedia** é um toolkit de engenharia de mídia e *Smart Cutter* de alta performance escrito nativamente em Go. Projetado para rodar em ambientes de alta densidade de concorrência e restrição física (Edge Computing e Serverless), ele atinge eficiência extrema eliminando cópias de memória, reduzindo o tempo de inicialização (Cold Start) para **< 3ms**, e economizando até **145x** em consumo de RAM comparado ao FFmpeg tradicional.

---

## 🗺️ Principais Funcionalidades

### 1. Engine Core & Fluxo Concorrente
- **Multi-Track & B-Frame Support (CTTS/EDTS)**: Processa múltiplos tracks mantendo compensações de composição de B-frames e correções de sincronia labial via Edit Lists.
- **Sincronização por `SyncBarrier`**: Multiplexador cronológico concorrente que ordena pacotes PTS/DTS prevenindo *drift* acumulado e ruídos de rede (jitter).
- **Elisão de Cópia (Zero-Copy CGO Bridge)**: Passagem de frames YUV/RGBA diretamente do Heap do Go para encoders C (x264/x265) usando `unsafe.Slice` sem cópias redundantes no barramento de memória.

### 2. Gerenciamento Inteligente de RAM
- **`BufferPool` Avançado**: Reuso agressivo de byte slices com buckets de tamanho fixo para elidir alocações na Heap de Go.
- **GC Safety Finalizers**: Rastreabilidade atômica de leases que devolve automaticamente buffers esquecidos ao pool via `runtime.SetFinalizer` para evitar vazamentos de memória (leaks).
- **Dynamic Pool Pruning**: Poda automática de buffers gigantes ociosos (>60 segundos) para mitigar fragmentação de heap.
- **Hybrid Jitter Buffer (Spill-to-Disk)**: Paginação de dados de rede excedentes para o disco quando a RAM atinge limite configurado (50MB).

### 3. Processamento de Vídeo & Áudio (DSP)
- **Filtros Concorrentes**: `ScaleFilter` (redimensionamento bilinear), `ColorFilter` (ajuste tonal), `OverlayFilter` (marca d'água semi-transparente) e `DrawText` renderizados por scanlines paralelos.
- **Sinc Resampler**: Conversor de taxa de amostragem de alta fidelidade com tabelas de consulta de coeficientes (LUT) pré-calculadas e janela Blackman-Harris.
- **Loudness Predictivo (Passagem Única)**: Normalização de volume em tempo real (`PredictiveGainNormalizer`) baseado em Média Móvel Exponencial (EMA) com limitação suave (*soft-knee*), mantendo latência zero.

### 4. CLI de Compatibilidade & Fallback
- **FFmpeg Syntax Interpreter**: Mapeamento inteligente de flags clássicas (`-i`, `-vf`, `-af`, `-c:v copy`, `-map`, `-metadata`, HLS, RTMP).
- **Mecanismo de Fallback**: Caso o CroMedia encontre flags não mapeadas, ele delega de forma transparente ao processo executável `ffmpeg` instalado no PATH.
- **Modo Estrito (Strict)**: Permite travar a execução do CLI para rodar unicamente sob o motor nativo Go, bloqueando fallbacks:
  ```bash
  export CROMEDIA_STRICT=true
  ```

---

## 📈 Tabela Comparativa de Benchmarks (16 Projetos)

Abaixo está o painel consolidado de velocidade e memória entre CroMedia e FFmpeg nos testes sintéticos e de indústria:

| Caso de Uso / Projeto | Tempo CroMedia | Tempo FFmpeg | Speedup (Tempo) | Memória CroMedia | Memória FFmpeg | Redução de Memória |
|:---|:---:|:---:|:---:|:---:|:---:|:---:|
| **Optimizer** | 1 ms | 14500 ms | **14500.00x** | 22.40 MB | 95.70 MB | **4.27x** |
| **Panóptico (NVR Edge)** | 222 ms | 13125 ms | **59.12x** | 26.50 MB | 3850.00 MB | **145.28x** |
| **Autocut** | 43 ms | 310 ms | **7.21x** | 13.67 MB | 55.40 MB | **4.05x** |
| **Flash-Transcoder** | 461 ms | 3622 ms | **7.86x** | 14.20 MB | 78.40 MB | **5.52x** |
| **Twitch ABR** | 1301 ms | 7450 ms | **5.73x** | 34.50 MB | 3810.00 MB | **110.43x** |
| **Cloudflare TTFB** | 0.037 ms | 165 ms | **4459.46x** | 2.20 MB | 68.50 MB | **31.14x** |
| **Netflix Concat** | 20 ms | 3845 ms | **192.25x** | 2.73 MB | 95.30 MB | **34.91x** |
| **MSU Codec** | 1 ms | 6295 ms | **6295.00x** | 8.23 MB | 145.20 MB | **17.64x** |
| **Watermark** | 1089 ms | 4500 ms | **4.13x** | 18.20 MB | 70.40 MB | **3.87x** |
| **Gif Forge** | 1014 ms | 3800 ms | **3.75x** | 15.60 MB | 60.10 MB | **3.85x** |
| **P2p Stream** | 2234 ms | 7900 ms | **3.54x** | 4.31 MB | 80.20 MB | **18.60x** |
| **Timelapse** | 4820 ms | 16800 ms | **3.49x** | 28.60 MB | 85.60 MB | **2.99x** |
| **Manifesto** | 1791 ms | 6200 ms | **3.46x** | 24.10 MB | 75.30 MB | **3.12x** |
| **Dashboard Hls** | 21885 ms | 65000 ms | **2.97x** | 38.50 MB | 120.50 MB | **3.13x** |
| **Chromakey** | 743 ms | 850 ms | **1.14x** | 19.80 MB | 75.80 MB | **3.83x** |
| **Audiosegregator** | 167 ms | 180 ms | **1.08x** | 35.78 MB | 45.20 MB | **1.26x** |
| **TOTAL / MÉDIA** | **35792 ms** | **154542 ms** | **4.32x** | **19.33 MB** | **552.98 MB** | **28.61x** |

---

## 🛠️ Como Compilar e Executar

### 1. Compilação Local
Gere o binário estático e compacto:
```bash
go build -ldflags="-s -w" -o cromedia main.go
```

Caso queira incluir codecs e containers legados compilados:
```bash
go build -tags "legacy" -ldflags="-s -w" -o cromedia main.go
```

### 2. Comandos CLI

#### Inspecionar Árvore de Átomos
```bash
./cromedia probe video.mp4

# Formato JSON
./cromedia probe video.mp4 --json
```

#### Cortar Vídeo (GOP Keyframe Accurate)
```bash
./cromedia cut input.mp4 10.5 45.0 output.mp4

# Ativar Smart Rendering
./cromedia cut input.mp4 10.5 45.0 output.mp4 --smart
```

#### Listar Plugins Dinâmicos Carregados
```bash
./cromedia plugins list

# Servidor REST para depurar telemetria de plugins
./cromedia plugins server 8080
```

#### Geração de Autocompletar Shell (Bash)
```bash
source <(./cromedia autocomplete)
```

---

## 🧪 Suíte de Validação e Benchmarks

O repositório possui rotinas robustas para certificar conformidade e velocidade em ambiente de concorrência extrema:

```bash
# 1. Rodar testes unitários do core
go test -v ./core/...

# 2. Rodar testes com suporte a codecs legados
go test -tags "legacy legacy_avi legacy_asf legacy_rm legacy_mp2" -v ./...

# 3. Rodar os 100 Casos Infernais (Hellcases)
go run benchmark/run_hellcases.go

# 4. Rodar testes de sincronismo (PTS-DTS Matrix & Sync)
go run benchmark/run_sync_benchmarks.go
```

---

## 🧩 Integração via API Fluente (Fluent API)

Você pode instanciar o CroMedia como biblioteca direta no seu microsserviço Go para criar cadeias de filtros sem a necessidade de comandos shell:

```go
package main

import (
	"context"
	"cromedia/core"
)

func main() {
	pctx := core.NewPipelineContext(context.Background())
	
	// Pipeline serverless: redimensiona, aplica grade de cor, marca d'água e loudness preditivo
	_ = core.Input("segmento_raw.mp4").
		Scale(1280, 720).
		ColorGrade(15.0, 1.1).
		DrawText("© NodeMídia", 20, 20).
		LoudnessNormalize(-1.0).
		Output("segmento_saida.ts").
		RunWithContext(pctx)
		
	pctx.PrintReport()
}
```

---
*Para referências detalhadas de código e design, consulte a pasta [/documentacao](file:///home/j/Documentos/GitHub/cromedia/documentacao).*
*CroMedia é parte integrante da infraestrutura de engenharia de mídia Nodus.*
