# CroMedia v0.8 🚀

O CroMedia é um Smart Cutter de alta performance escrito puramente em Go. Ele foca em extração e remuxing de MP4 sem re-encodificação (bit-stream copy), garantindo velocidade máxima e zero perda de qualidade.

A versão v0.8 eleva o CroMedia de um script utilitário para uma ferramenta de engenharia de mídia profissional, com suporte a multi-track, entrelaçamento otimizado para web e correção de sincronia via Edit Lists.

## Principais Funcionalidades (v0.8)

- **Multi-Track Support**: Processa vídeo e áudio simultaneamente, mantendo múltiplos fluxos sincronizados.
- **Web-Optimized Interleaving**: Entrelaçamento de samples baseado em timestamp (Fast Start), permitindo reprodução instantânea via streaming.
- **B-Frame Support (CTTS)**: Mantém a ordem de composição correta para vídeos que utilizam B-frames.
- **Edit List Support (EDTS/ELST)**: Preserva e aplica correções de sincronia labial (lip-sync) e offsets de áudio/vídeo.
- **Matrix Rotation Copy**: Preserva a orientação original (ex: vídeos verticais de iPhone) copiando a matriz de transformação do `tkhd`.
- **co64 Support**: Suporte automático para arquivos gigantes (>4GB) usando offsets de 64 bits.
- **Bit-Stream Copy**: Zero re-encodificação. O corte é feito diretamente nos Keyframes (I-Frames).

## Como Usar

### Instalação
```bash
go build -o cromedia main.go
```

### Comandos

#### Inspecionar Árvore de Átomos
```bash
./cromedia probe video.mp4
```

#### Cortar Vídeo (Keyframe Accurate)
```bash
./cromedia cut input.mp4 <inicio_seg> <fim_seg> output.mp4
```
*Exemplo: `./cromedia cut clipe.mp4 10.5 25.0 output.mp4`*

#### Ver Versão e Features
```bash
./cromedia version
```

## Arquitetura

O CroMedia foi projetado para ser eficiente em memória e CPU:
- **Demuxer**: Parser recursivo de baixo nível para a estrutura de átomos (ISO BMFF).
- **Cutter**: Algoritmo de busca por keyframes com relatório delta (exibe o ajuste exato feito no corte).
- **Remuxer**: Estratégia de escrita em dois passos para mdat (streaming via `io.Copy`) e moov (in-memory).

---
*CroMedia é parte da engine de processamento de mídia do ecossistema Nodus.*
