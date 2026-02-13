# Rumo à v1.0: O Plano "FFmpeg Killer" 🛠️

Embora a v0.8 esteja estável e robusta para uso em produção (copy-mode), o caminho para a v1.0 envolve desafios técnicos de nível extremo para transformar o CroMedia no cortador de vídeo definitivo.

## 1. Smart Rendering (`--smart`)

Atualmente, o CroMedia só corta em Keyframes (GOP boundaries). Se você pedir um corte no segundo 3.5 e o keyframe estiver no 0.0, o CroMedia incluirá os 3.5s extras.

**O Objetivo:**
Integrar um encoder (x264/OpenH264) via CGO para permitir cortes **frame-perfect**.

**Lógica de Implementação:**
1. **Decode do GOP inicial**: Decodifica apenas o primeiro grupo de imagens afetado pelo corte.
2. **Trim Exato**: Descarta os frames indesejados.
3. **Encode de transição**: Codifica um novo GOP inicial que termina exatamente onde começa o próximo keyframe original.
4. **Muxing Híbrido**: Faz o "Stitch" do novo GOP codificado com o resto do vídeo original em modo *Copy*.

## 2. Suporte a WebM / VP9 / AV1

O CroMedia v0.8 é um especialista em MP4 (norma ISO BMFF/isom). No entanto, o futuro da web e o streaming moderno exigem outros formatos.

**Desafio:**
- A estrutura de **Atoms** (MP4) é fundamentalmente diferente da estrutura **EBML** utilizada em Matroska (MKV) e WebM.
- Será necessário criar um novo parser e remuxer específico para fluxos VP9 e AV1.

## 3. Aceleração de Hardware Real (GPU)

Atualmente, o diretório `core/hardware/` contém stubs (simulações) baseadas na API NVENC da NVIDIA (`nvenc_linux.go`).

**O Objetivo:**
- Implementar as chamadas reais via CGO para `libnvidia-encode`.
- Permitir que o *Smart Rendering* (Fase 1) ocorra em milissegundos usando a GPU, mantendo a CPU livre para outras tarefas.
- Suporte a decodificação via NVDEC/VAAPI.

## 4. Precisão de Áudio (Sub-frame Trimming)

O áudio AAC em MP4 vem em pacotes de 1024 samples. Atualmente, o CroMedia inclui o pacote inteiro se o corte cair no meio dele.

**O Objetivo:**
- Implementar o ajuste de `Implicit Reconstruction` no final do arquivo ou re-encodagem pontual dos frames de áudio de borda para garantir que a duração do áudio bata perfeitamente (milissegundo por milissegundo) com a duração do vídeo.

---

### Status do Placeholder `--smart`
A flag `--smart` já presente no `main.go` da v0.8 é um sinalizador futuro. Ativá-la hoje apenas exibe uma mensagem de intenção, servindo como base arquitetural para onde as novas bibliotecas de codec serão injetadas.
