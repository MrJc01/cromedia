# Estudo de Alternativas e Foco Estratégico do CroMedia 📊

Para tornar o CroMedia uma alternativa viável, moderna e performática ao FFmpeg, é fundamental entender o histórico de outros projetos do ecossistema Go/Rust, analisando seus acertos, erros e limitações.

---

## 🔍 1. Análise de Projetos Similares

### A. Joy4 (Go)
O **Joy4** foi uma das primeiras tentativas ambiciosas de criar um framework de processamento de áudio/vídeo puro em Go com suporte a RTMP, RTSP, MP4 e FLV.
*   **Onde acertou:** Criou interfaces de demuxer/muxer limpas e fáceis de integrar em Go.
*   **Onde falhou:**
    1.  **Falta de Manutenção:** O projeto não acompanhou a evolução de novos codecs (H.265/AV1) e containers modernizados.
    2.  **Problemas de Performance de Memória:** Alocações contínuas de arrays de bytes para frames geravam picos de Garbage Collection (GC), tornando-o instável para processamento de alto throughput.
    3.  **Acoplamento Frágil com CGO:** A ponte para decodificadores baseados em FFmpeg era instável e de difícil compilação.

### B. MediaMTX / Gortsplib (Go)
O **MediaMTX** (antigo `rtsp-simple-server`) é um caso de **sucesso extremo** escrito em Go. Ele roteia fluxos RTSP, RTMP, WebRTC, HLS e SRT.
*   **Onde acertou:**
    1.  **Foco em Rede e Muxing:** Ele faz apenas o demuxing e muxing dos pacotes (cópia de fluxo de bits) nativamente em Go puro (`gortsplib`).
    2.  **Evitou Decodificação:** Ele **não tenta decodificar ou codificar os frames na CPU usando Go**. Se o usuário precisa de transcodificação, ele orienta a execução de um subprocesso externo do FFmpeg.
    3.  **Portabilidade:** Compila para um binário único estático e altamente performático.

### C. Astiav / Goav (Go bindings para FFmpeg)
Bibliotecas que criam bindings CGO diretas sobre a `libavcodec`/`libavformat` do FFmpeg.
*   **Onde acertou:** Permitem utilizar o poder total do FFmpeg sem sair do Go.
*   **Onde falhou/Sofre limitações:**
    1.  **Inferno de Dependências:** Exige que a máquina de compilação e execução tenha o FFmpeg instalado (`libavutil-dev`, etc.), invalidando o benefício de binário estático do Go.
    2.  **Overhead de CGO:** Chamar funções C para cada frame/pacote de vídeo em laços quentes (hot loops) adiciona latência devido à transição de contexto do scheduler do Go para C.
    3.  **Erros Catastróficos (Segfaults):** Qualquer estouro de buffer ou ponteiro nulo no lado C derruba todo o processo Go, sem possibilidade de recuperação via `recover()`.

### D. Symphonia (Rust)
Uma biblioteca open-source escrita em Rust puro focada em demuxing e decodificação de áudio.
*   **Onde acertou:** Conseguiu implementar decodificadores estáveis e extremamente rápidos (MP3, AAC, FLAC, Opus) de forma segura em Rust puro.
*   **Onde falhou:** Focou apenas em áudio. A complexidade matemática e o licenciamento de decodificadores/codificadores de vídeo (H.264/H.265) inviabilizaram uma expansão rápida para vídeo puro.

---

## 🎯 2. Onde o CroMedia deve Focar para Vencer?

Escrever decodificadores de vídeo completos em Go puro (especialmente para H.264, H.265 e AV1) é um trabalho hercúleo e uma armadilha que consome anos de desenvolvimento com performance inferior a implementações C/Assembly altamente otimizadas por SIMD.

Portanto, o CroMedia deve focar em **cinco pilares estratégicos** para se diferenciar do FFmpeg tradicional:

```
+--------------------------------------------------------------------------+
|                     PILARES DE FOCO DO CROMEDIA                          |
+--------------------------------------------------------------------------+
|  1. Mux/Demux em Go Puro (Copy-Mode)                                     |
|  2. Smart Cutting (Transcodificação Híbrida de Borda)                     |
|  3. CGO Opcional via Build Tags (Módulos Plugáveis)                      |
|  4. Reaproveitamento de Memória (Zero Alloc / sync.Pool)                 |
|  5. Foco em Codecs Modernos (H.264, H.265, AV1, AAC, Opus)               |
+--------------------------------------------------------------------------+
```

### 1. Mux/Demux de Containers em Go Puro (Copy-Mode Primeiro)
Toda a leitura da estrutura lógica dos arquivos (parsing de átomos do MP4, pacotes EBML do WebM/MKV e pacotes de 188 bytes do MPEG-TS) deve ser feita em Go puro.
*   **Benefício:** Operações como extração de áudio, troca de container (ex: MP4 para TS) ou corte em Keyframes rodam sem CGO, sem dependências externas, com performance absurda e portabilidade total.

### 2. Smart Cutting (Transcodificação Híbrida de Borda)
Em vez de re-encodar o vídeo inteiro para fazer um corte de precisão de milissegundos, o CroMedia focará na **Re-encodagem de GOP de Borda**:
*   Decodifica e re-encoda apenas as frações de segundo antes do primeiro keyframe e depois do último keyframe.
*   Todo o miolo do vídeo (que pode durar minutos ou horas) é copiado diretamente (*bit-stream copy*).
*   **Resultado:** Cortes perfeitos em frações de segundo, gastando 1% da CPU de um FFmpeg transcodificando por completo.

### 3. CGO Opcional e Módulos Plugáveis via Build Tags
Diferente das bindings tradicionais que amarram todo o projeto a dependências de C, o CroMedia deve usar `go:build` para separar os encoders:
*   Sem tags (padrão): Compila um binário Go estático super leve que realiza apenas demux, remux, metadata probe, chapters e keyframe cutting.
*   Com tags (`-tags "nvidia openh264"`): Ativa a transcodificação inteligente via hardware (NVENC) ou via software encoder.

### 4. Reaproveitamento de Memória (Zero Alloc)
Para evitar o problema do Joy4 com o Garbage Collector do Go:
*   Utilização agressiva de `sync.Pool` para buffers de vídeo/áudio decodificados.
*   Reutilização de estruturas de pacotes durante o fluxo de dados do pipeline, evitando alocações Heap em laços de repetição rápidos.

### 5. Foco Restrito a Formatos Modernos de Web/Streaming
O FFmpeg é pesado porque suporta centenas de formatos legados da década de 90 (ASF, RealMedia, Sorenson Spark, MP3, etc.). O CroMedia focará estritamente no ecossistema moderno:
*   **Containers:** MP4/fMP4, WebM, Matroska (MKV), MPEG-TS.
*   **Codecs de Vídeo:** H.264 (AVC), H.265 (HEVC), VP9, AV1.
*   **Codecs de Áudio:** AAC, Opus, PCM.
*   **Legendas:** SRT, WebVTT.
*   Qualquer codec legado será ignorado para manter a base de código limpa e o binário pequeno.
