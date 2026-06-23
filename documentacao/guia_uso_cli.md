# Guia de Uso da CLI (Interface de Linha de Comando) 💻

O executável do CroMedia provê uma interface robusta no terminal contendo comandos utilitários, telemetria detalhada e um modo de compatibilidade com a sintaxe tradicional do FFmpeg.

---

## 🚀 Instalação e Compilação

Para compilar o binário localmente com otimizações de linkagem:

```bash
go build -ldflags="-s -w" -o cromedia main.go
```

Para incluir codecs e demuxers legados compilados de forma estática:

```bash
go build -tags "legacy" -ldflags="-s -w" -o cromedia main.go
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

# Ativar renderização inteligente para GOP boundaries (re-encoda apenas as bordas do corte)
./cromedia cut input.mp4 12.5 45.0 output.mp4 --smart
```

### 3. `plugins` (Gerenciamento Dinâmico de Plugins)
Lista os plugins de codecs carregados em tempo de execução via arquivos dinâmicos `.so` ou `.dll`.

```bash
# Listar plugins disponíveis por categoria (Decoders, Encoders, Demuxers, Muxers)
./cromedia plugins list

# Inicializar um servidor HTTP REST local de depuração de plugins na porta 8080
./cromedia plugins server 8080
```
API exposta: `GET http://localhost:8080/debug/plugins` retorna JSON detalhado com cotas, uso de CPU e memória por plugin.

### 4. `devices` (Dispositivos Gráficos)
Lista os dispositivos de aceleração de hardware disponíveis e stubs de fallbacks compilados.

```bash
./cromedia devices
```

### 5. `codecs` (Lista de Codecs)
Lista todos os codecs registrados no runtime (nativos Go e wrappers externos CGO).

```bash
./cromedia codecs
```

### 6. `formats` (Formatos Mapeados)
Lista todos os demuxers e muxers habilitados para gravação e leitura.

```bash
./cromedia formats
```

### 7. `autocomplete` (Geração de Autocompletar Shell)
Gera scripts de autocompletar para shell Bash facilitando o desenvolvimento interativo.

```bash
# Carregar autocompletar na sessão atual do Bash
source <(./cromedia autocomplete)
```

---

## 🔄 Compatibilidade com FFmpeg & Modo Fallback

O CroMedia intercepta chamadas no formato tradicional do FFmpeg, parseando parâmetros de entrada, bitrates, filtros complexos (`-vf`, `-af`, `-filter_complex`), capítulos, metadados e destinos.

```bash
# Executar conversão com filtros de vídeo clássicos
./cromedia -i input.mp4 -ss 10.0 -to 20.0 -vf "scale=1280:720,drawtext=text='© CroMedia'" -c:v h264 -y output.mp4
```

### ⚠️ Lógica de Fallback Automático e Estrito
Caso o analisador do CroMedia detecte flags incompatíveis ou codecs C legados indisponíveis na compilação Go, o CLI repassa elegantemente a chamada para o executável `ffmpeg` instalado no PATH:
* **Log de Aviso**: Um `WARNING` é exibido indicando que a tarefa foi delegada ao FFmpeg.
* **Modo Estrito (Strict Mode)**: Se você deseja bloquear o fallback automático e garantir que apenas o motor CroMedia Go-native execute o comando:
  ```bash
  # Bloquear via flag CLI
  ./cromedia --strict -i input.mp4 -ss 10 -t 5 output.mp4
  
  # Bloquear globalmente via variável de ambiente
  export CROMEDIA_STRICT=true
  ```

### ⏱️ Profiling com `--benchmark`
Para fins de depuração de latência e alocação de recursos, anexe a flag `--benchmark` ao comando:
```bash
./cromedia -i input.mp4 -vf "scale=640:480" output.mp4 --benchmark
```
A saída exibirá o tempo gasto pelo analisador, tempo de execução do pipeline (ou subprocesso) e o pico de consumo de RAM.
