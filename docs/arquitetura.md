# Arquitetura do CroMedia 🛠️

O CroMedia foi projetado seguindo uma arquitetura modular baseada em pipelines de fluxo de dados (dataflow streams).

## Componentes Principais

```
            +---------------------------------------+
            |              Input File               |
            +---------------------------------------+
                                |
                                v
            +---------------------------------------+
            |               Demuxer                 |
            |   (Parsers nativos: MP4, WebM, etc.)  |
            +---------------------------------------+
              /           |           \         \
    Video Samples   Audio Samples   Metadata   Chapters
            /             |             \         \
           v              v              v         v
    +-------------+ +-------------+ +---------------------+
    | Video Dec   | | Audio Dec   | |   Metadata Parser   |
    +-------------+ +-------------+ +---------------------+
           |              |                    |
           v              v                    |
    +-------------+ +-------------+            |
    | Video Filt  | | Audio Filt  |            |
    +-------------+ +-------------+            |
           |              |                    |
           v              v                    |
    +-------------+ +-------------+            |
    | Video Enc   | | Audio Enc   |            v
    +-------------+ +-------------+   +-------------------+
           \              |          /|   Header Generator|
            \             |         / +-------------------+
             v            v        v           |
            +------------------------+         |
            |        Muxer           |<--------+
            | (Reconstrução e Mux)   |
            +------------------------+
                        |
                        v
            +---------------------------------------+
            |              Output File              |
            +---------------------------------------+
```

### 1. Demuxers e Muxers (Nativos em Go)
*   **Demuxer**: Responsável por analisar a estrutura do arquivo container (ex: átomos do MP4, blocos EBML do WebM) e extrair os pacotes (samples) de vídeo, áudio e legendas, bem como tabelas de sincronização (`stts`, `stco`, `ctts`, etc.).
*   **Muxer**: Reconstrói a estrutura do container a partir de fluxos brutos de pacotes intercalados, gerando índices válidos para reprodução rápida (Fast Start).

### 2. Pipeline de Processamento e Agendador (`Scheduler`)
*   O processamento é baseado em **GOPs (Group of Pictures)**.
*   Um agendador (`Scheduler`) e um pool de workers concorrentes (`WorkerPool`) distribuem os GOPs para processamento paralelo, otimizando o uso de múltiplos núcleos da CPU.

### 3. Decodificadores e Codificadores (Módulos Plugáveis CGO/Nativos)
*   Para operações que exigem modificação do fluxo de bits (como redimensionamento de vídeo, filtros de áudio, marca d'água), os pacotes passam por codecs decodificadores, filtros e depois codificadores.
*   O CroMedia suporta hooks para codecs via **CGO** (ex: `nvenc` para NVIDIA, `openh264` para H.264) mantendo a estrutura limpa e passível de compilação sem esses módulos (via build tags).

### 4. Smart Cutter (Corte Inteligente)
*   Minimiza a transcodificação.
*   Se o usuário solicita um corte que não coincide exatamente com um Keyframe (I-Frame), o CroMedia re-encoda apenas a transição inicial (primeiro GOP) e final (último GOP), enquanto copia diretamente os GOPs intermediários íntegros (*bit-stream copy*).

### 5. Arquitetura Modular de Plugins (Dinâmico e Estático)
*   **Plugins Dinâmicos**: Carregamento sob demanda de arquivos `.so` (Linux) ou `.dll` (Windows) em runtime com validação criptográfica e SemVer/ABI.
*   **Isolamento (Sandboxing)**: Execução opcional em subprocesso isolado via GOB IPC, protegendo contra vazamentos de memória e falhas graves.
*   **Plugins Legados Estáticos**: Módulos legados (ASF, AVI, RealMedia, MP2 e codecs obsoletos) integrados sob build tags (ex: `-tags "legacy"`) para compilação estática opcional, preservando o tamanho mínimo do núcleo.

### 6. Módulo e Sequenciador de Imagens Nativas
*   Manipulação de formatos de imagem (PNG, JPEG, WebP, BMP, TIFF) com suporte a sniffer de bytes mágicos.
*   Leitura de sequências de imagens do disco utilizando padrões Glob ou Printf para montagem de fluxo de vídeo e exportação de frames intercalados.

### 7. Sistema Central de Filtros de Áudio e Vídeo
*   Gerenciamento dinâmico e thread-safe de filtros via fábrica de criação (`core/filters/filter_mgr.go`).
*   Filtros nativos implementados (ajustes tonais, desfoque/nitidez, keying de croma, equalizadores paramétricos multibanda, etc.) complementados por pontes opcionais CGO para a `libavfilter` do FFmpeg.

### 8. CLI FFmpeg-Compat e Mecanismo de Fallback
*   Interface compatível com a sintaxe clássica do FFmpeg (flags `-i`, `-vf`, `-af`, `-c:v`, etc.).
*   Fallback transparente para a instalação local do `ffmpeg` com barra de progresso em linha e controle de execução estrita (`--strict` / `CROMEDIA_STRICT`).

