# Guia de Uso da CLI (Interface de Linha de Comando) 💻

O executável do CroMedia provê uma interface robusta no terminal contendo comandos utilitários e um modo de compatibilidade com a sintaxe tradicional do FFmpeg.

---

## 🚀 Instalação e Compilação

Para compilar o binário localmente com otimizações de linkagem:

```bash
go build -ldflags="-s -w" -o cromedia main.go
```

---

## 🛠️ Comandos Disponíveis

### 1. `probe` (Inspecionar Átomos de MP4/fMP4)
Varre e imprime a árvore de átomos do container sem carregar os payloads pesados de vídeo.

```bash
# Imprimir árvore estruturada com recuo visual
./cromedia probe video.mp4

# Retornar árvore de átomos em formato JSON
./cromedia probe video.mp4 --json
```

### 2. `cut` (Corte Rápido de Vídeo Sem Perdas)
Realiza fatiamento de múltiplos tracks de vídeo e áudio utilizando snapping para keyframe em complexidade de tempo $O(\log N)$.

```bash
./cromedia cut input.mp4 12.5 45.0 output.mp4

# Ativar renderização inteligente para GOP boundaries
./cromedia cut input.mp4 12.5 45.0 output.mp4 --smart
```

### 3. `devices` (Dispositivos Gráficos)
Lista os dispositivos de aceleração de hardware disponíveis e stubs de fallbacks compilados.

```bash
./cromedia devices
```

### 4. `codecs` (Lista de Codecs)
Lista todos os codecs registrados no runtime (nativos Go e wrappers externos CGO).

```bash
./cromedia codecs
```

### 5. `formats` (Formatos Mapeados)
Lista todos os demuxers e muxers habilitados para gravação e leitura.

```bash
./cromedia formats
```

---

## 🔄 Compatibilidade com FFmpeg

O CroMedia intercepta chamadas no formato tradicional do FFmpeg, parseando parâmetros de input, codecs, offsets temporais e destinos.

```bash
# Executar corte no formato clássico
./cromedia -i input.mp4 -ss 00:01:00 -t 10 -c:v copy output.mp4
```
