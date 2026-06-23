# CroMedia: Uma Alternativa Leve e de Alta Performance ao FFmpeg 🚀

## O Desafio do FFmpeg
O FFmpeg é a ferramenta definitiva para manipulação de mídia, porém carrega consigo algumas desvantagens para cenários modernos de microsserviços, serverless e processamento na nuvem:
1. **Monolito Gigante**: Possui um binário muito pesado contendo centenas de codecs legados ou raramente utilizados.
2. **Uso de Recursos**: Alto consumo de memória e CPU por padrão, com dificuldades em isolar ou otimizar pipelines específicos.
3. **Complexidade de Integração**: Integrar FFmpeg em Go geralmente exige chamadas pesadas de subprocessos (`exec.Command`) ou bindings CGO complexas para a `libav*`.
4. **Startup Time**: Overhead de inicialização de processos externos que impacta arquiteturas serverless (ex: AWS Lambda).

## A Visão do CroMedia
A proposta do CroMedia é ser um motor de processamento de mídia **modular, altamente concorrente e leve**, escrito em **Go**, capaz de realizar as tarefas mais comuns do FFmpeg (demuxing, remuxing, smart cutting, transcoding, filtragem e streaming) de forma mais eficiente e nativa.

### Pilares de Design:
*   **Go Nativo**: Sempre que possível, decodificação de metadados, demuxing e remuxing serão feitos em Go puro para máxima segurança de memória e concorrência nativa (goroutines).
*   **Modularidade Total**: Codecs pesados (como H.264/AV1) e filtros serão módulos opcionais, permitindo compilar binários customizados extremamente enxutos.
*   **Smart Processing**: Priorizar sempre operações de cópia de fluxo de bits (*bit-stream copy*) e aplicar transcodificação inteligente apenas nos frames estritamente necessários (GOP boundaries).
*   **Pronto para Nuvem**: Desenvolvido sob medida para processamento paralelo de mídia em pipelines serverless, com baixo tempo de inicialização e baixo consumo de memória estável.
