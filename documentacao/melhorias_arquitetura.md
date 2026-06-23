# Melhorias de Arquitetura Baseadas na Análise dos Especialistas 🚀

Este documento consolida as sugestões de melhorias arquiteturais para o CroMedia, endereçando diretamente os pontos críticos e limitações apontados no comitê de debate dos 30 especialistas em engenharia de mídia.

---

## 🏗️ 1. Runtime Go, Concorrência e Backpressure

### Crítica do Painel
- **Contenção do Scheduler**: Alta densidade de fluxos simultâneos pode sobrecarregar o escalonador M:N de Goroutines.
- **Deadlocks sob Cancelamento**: Se uma thread bloquear no envio de dados, pode travar todo o pipeline.

### Melhorias Propostas
1. **Worker Pools Hierárquicos**: Em vez de disparar Goroutines ad-hoc para cada GOP em trânsito, implementar um pool global compartilhado por múltiplos pipelines ativos para estabilizar o agendador de Go.
2. **Timeouts e Context Enforcement**: Adicionar cláusulas de timeout (`select { case <-ctx.Done(): ... }`) em todas as operações de envio e recebimento de canais do pipeline, garantindo que qualquer pipeline sob backpressure destrave em no máximo 50ms se o contexto expirar (como validado no teste unitário `TestExpertLeakAndDeadlockValidation`).

---

## 📦 2. Gerenciamento de Memória & Prevenção de Leaks

### Crítica do Painel
- **Memory Leaks Silenciosos**: Buffers que escapam do pool global de buffers (`core.GlobalPut`) causam vazamentos de longo prazo.
- **Fragmentação no Heap por Resoluções Dinâmicas**: Alterações abruptas de tamanhos de fatias podem inflar a memória do host.

### Melhorias Propostas
1. **Atomic Allocation Trackers (Rastreabilidade)**: Integrar identificadores de ciclo e contadores atômicos em cada array obtido via `GlobalGet`. Se um buffer ficar ativo por mais de 5 segundos, uma rotina em background reporta um alerta visual de leak.
2. **Garbage Collection Fallback via Finalizers**: Associar `runtime.SetFinalizer` a structs internas de buffers para que, mesmo se o desenvolvedor esquecer de chamar o método `.Close()` ou liberar o buffer, o próprio runtime do Go intercepte e devolva o array ao pool.
3. **Dynamic Pool Pruning (Poda)**: Implementar uma rotina de expiração que descarta fatias de memória gigantescas (usadas em resoluções altas) se ficarem ociosas no pool por mais de 60 segundos.

---

## 📹 3. Otimizações de Vídeo e Processamento Digital (SIMD/AVX)

### Crítica do Painel
- **Falta de Vetorialização de Hardware**: Em processamentos puros de vídeo 4K (e.g. Sobel, bilinear scale), loops Go nativos são mais lentos que instruções vetorizadas assembly (AVX-512) usadas no FFmpeg.
- **Overhead da Pilha CGO**: Chamadas de frames H.264/H.265 através da fronteira C-Go adicionam sobrecarga de pilha.

### Melhorias Propostas
1. **Assembly Go para Loops de Pixels**: Escrever as rotinas críticas de conversão de pixel YUV para RGBA e filtragem Sobel utilizando assembly Go nativo com instruções SIMD (AVX2/AVX-512) para maximizar o throughput da CPU.
2. **Processamento em Batch (CGO Buffer Packing)**: Agrupar frames em pedaços e enviá-los de uma só vez para os codificadores baseados em C (como x264), reduzindo o número de transições de contexto C-Go por frame.

---

## 🔊 4. Áudio de Alta Fidelidade (DSP)

### Crítica do Painel
- **Artefatos de Aliasing no Resampler**: A interpolação linear gera distorções harmônicas de alta frequência se comparada a filtros sinc.
- **Normalização com Latência**: O Peak Normalizer exige passagem dupla (double pass) introduzindo atraso de áudio.

### Melhorias Propostas
1. **Sinc Resampling Otimizado**: Implementar filtros sinc baseados em tabelas de consulta de coeficientes pré-calculados (Look-Up Tables - LUT) para acelerar a precisão sem degradar a CPU.
2. **Normalização em Tempo Real (Gain Estimators)**: Implementar estimadores de ganho preditivos baseados em estatísticas históricas dos frames anteriores para normalizar a amplitude em uma única passagem (single-pass), reduzindo a latência a zero.

---

## 🌐 5. Streaming de Rede & Distribuição

### Crítica do Painel
- **Transbordamento do Jitter Buffer**: Buffers na RAM podem estourar sob degradação severa da largura de banda.
- **Estabilidade do Alinhamento HLS**: CDNs rejeitam streams se PCR/PTS de áudio/vídeo apresentarem desvios acumulados.

### Melhorias Propostas
1. **Hybrid Jitter Buffer (Spill-to-Disk)**: Implementar mecanismo de paginação de pacotes excedentes para o disco local se o Jitter Buffer na RAM ultrapassar 50MB, preservando a integridade do stream.
2. **Clock Sync e Descontinuidade Estrita**: Sincronizar o relógio PCR no muxer MPEG-TS de forma contínua com margens de desvio de no máximo 500ns, disparando sinalizações de descontinuidade no arquivo `.m3u8` de forma preemptiva.
