# Guia da API Fluente (Fluent API) para Go 🧩

Para desenvolvedores que queiram integrar o CroMedia diretamente como biblioteca em seus projetos Go, o framework provê um construtor de pipelines fluente (`PipelineBuilder`).

---

## 🏗️ Como Importar

Importe o pacote core no seu arquivo `.go`:

```go
import "cromedia/core"
```

---

## 🛠️ Exemplo de Uso Prático

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

## ⚙️ Método de Funcionamento Interno

Ao chamar o método `.Run()`, o `PipelineBuilder` valida os caminhos, carrega o demuxer apropriado baseado no magic bytes sniff do cabeçalho, cria o grafo de filtros configurados, inicializa o `WorkerPool` concorrente e grava os pacotes resultantes respeitando a ordem de amostragem.
    
Todas as alocações de fatias temporárias são feitas consultando o pool de buffers global para garantir baixa alocação no heap de memória do Go.
