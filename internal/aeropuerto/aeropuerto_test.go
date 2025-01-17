package aeropuerto

import (
    "bufio"
    "fmt"
    "io"
    "net"
    "strings"
    "sync"
    "testing"
    "time"
)

// Estructura para almacenar resultados de pruebas
type ResultadoPrueba struct {
    tiempoTotal    time.Duration
    avionesExito   int
    avionesFallo   int
    tiempoPromedio time.Duration
}

// En aeropuerto_test.go

// Añadir constantes para controlar mejor los tiempos
const (
    timeoutPorAvion = 30 * time.Second
    timeoutTotal    = 60 * time.Second  // Reducido de 2 minutos a 1 minuto
    maxReintentos   = 5
)

// Estructura mejorada para el seguimiento de aviones
type EstadoAvion struct {
    avion      *Avion
    intentos   int
    procesado  bool
}

func ejecutarPrueba(numA, numB, numC int, t *testing.T) ResultadoPrueba {
    conn, err := net.Dial("tcp", "localhost:8000")
    if err != nil {
        t.Fatalf("Error conectando al servidor: %v", err)
    }
    defer conn.Close()

    // Reiniciar variables
    estado.valor = 0
    torre.avionesProcesando = 0
    avionesChannel = make(chan *Avion, maxEspera)

    // Mapa para seguimiento de aviones
    avionesMap := make(map[int]*EstadoAvion)
    var mutexMapa sync.Mutex
    //totalAviones := numA + numB + numC

    var wg sync.WaitGroup
    canalCerrado := false
    var mutexCanal sync.Mutex

    // Goroutine para procesar mensajes del servidor
    go func() {
        reader := bufio.NewReader(conn)
        for {
            mensaje, err := reader.ReadString('\n')
            if err != nil {
                if err == io.EOF {
                    break
                }
                continue
            }
            mensaje = strings.TrimSpace(mensaje)
            if mensaje != "" && !strings.HasPrefix(mensaje, "Aeropuerto localizado") {
                procesarMensajeServidor(mensaje)
            }
        }
    }()

    // Goroutine para procesar aviones
    wg.Add(1)
    go func() {
        defer wg.Done()
        for avion := range avionesChannel {
            wg.Add(1)
            go func(a *Avion) {
                defer wg.Done()
                
                mutexMapa.Lock()
                estadoAvion, existe := avionesMap[a.ID]
                if !existe {
                    estadoAvion = &EstadoAvion{avion: a, intentos: 0}
                    avionesMap[a.ID] = estadoAvion
                }
                estadoAvion.intentos++
                mutexMapa.Unlock()

                if estadoAvion.intentos > maxReintentos {
                    fmt.Printf("Avión %d (Cat %s) excedió máximo de reintentos\n", a.ID, a.Categoria)
                    return
                }

                done := make(chan bool)
                go func() {
                    procesarAvion(a)
                    done <- true
                }()

                select {
                case <-done:
                    mutexMapa.Lock()
                    estadoAvion.procesado = true
                    mutexMapa.Unlock()
                case <-time.After(timeoutPorAvion):
                    fmt.Printf("Avión %d (Cat %s) timeout en intento %d\n", a.ID, a.Categoria, estadoAvion.intentos)
                }
            }(avion)
        }
    }()

    inicio := time.Now()

    // Generar aviones con IDs únicos
    for i := 0; i < numA; i++ {
        avionesMap[i] = &EstadoAvion{
            avion: &Avion{ID: i, Categoria: "A", NumPasajeros: 101 + i},
        }
    }
    for i := 0; i < numB; i++ {
        avionesMap[numA+i] = &EstadoAvion{
            avion: &Avion{ID: numA + i, Categoria: "B", NumPasajeros: 50 + i},
        }
    }
    for i := 0; i < numC; i++ {
        avionesMap[numA+numB+i] = &EstadoAvion{
            avion: &Avion{ID: numA + numB + i, Categoria: "C", NumPasajeros: 30 + i},
        }
    }

    // Enviar aviones al canal
    for _, estadoAvion := range avionesMap {
        avionesChannel <- estadoAvion.avion
    }

    // Esperar a que se procesen todos los aviones o se alcance el timeout
    completado := make(chan bool)
    go func() {
        wg.Wait()
        completado <- true
    }()

    select {
    case <-completado:
        fmt.Println("Procesamiento completado normalmente")
    case <-time.After(timeoutTotal):
        fmt.Println("Test alcanzó tiempo máximo de ejecución")
    }

    // Cerrar el canal de forma segura
    mutexCanal.Lock()
    if !canalCerrado {
        close(avionesChannel)
        canalCerrado = true
    }
    mutexCanal.Unlock()

    // Calcular resultados finales
    avionesExito := 0
    avionesFallo := 0

    mutexMapa.Lock()
    for _, estadoAvion := range avionesMap {
        if estadoAvion.procesado {
            avionesExito++
        } else {
            avionesFallo++
        }
    }
    mutexMapa.Unlock()

    tiempoTotal := time.Since(inicio)
    var tiempoPromedio time.Duration
    if avionesExito > 0 {
        tiempoPromedio = tiempoTotal / time.Duration(avionesExito)
    }

    return ResultadoPrueba{
        tiempoTotal:    tiempoTotal,
        avionesExito:   avionesExito,
        avionesFallo:   avionesFallo,
        tiempoPromedio: tiempoPromedio,
    }
}

func TestCaso1DistribucionEquilibrada(t *testing.T) {
    resultados := ejecutarPrueba(10, 10, 10, t)
    
    fmt.Printf("\n=== Resultados Test Caso 1 (10A-10B-10C) ===\n")
    fmt.Printf("Tiempo total: %v\n", resultados.tiempoTotal)
    fmt.Printf("Aviones procesados con éxito: %d\n", resultados.avionesExito)
    fmt.Printf("Aviones no procesados: %d\n", resultados.avionesFallo)
    fmt.Printf("Tiempo promedio por avión: %v\n", resultados.tiempoPromedio)
    
    totalEsperado := 3 // 1A + 1B + 1C
    if resultados.avionesExito + resultados.avionesFallo != totalEsperado {
        t.Errorf("Número total de aviones incorrecto. Esperado: %d, Obtenido: %d", 
                 totalEsperado, resultados.avionesExito + resultados.avionesFallo)
    }
    
    if resultados.avionesFallo < 0 {
        t.Error("El número de aviones fallidos no puede ser negativo")
    }
}

// Test para el caso 2: mayoría categoría A
func TestCaso2MayoriaA(t *testing.T) {
    resultados := ejecutarPrueba(20, 5, 5, t)

    fmt.Printf("\n=== Resultados Test Caso 2 (20A-5B-5C) ===\n")
    fmt.Printf("Tiempo total: %v\n", resultados.tiempoTotal)
    fmt.Printf("Aviones procesados con éxito: %d\n", resultados.avionesExito)
    fmt.Printf("Aviones no procesados: %d\n", resultados.avionesFallo)
    fmt.Printf("Tiempo promedio por avión: %v\n", resultados.tiempoPromedio)
}

// Test para el caso 3: mayoría categoría C
func TestCaso3MayoriaC(t *testing.T) {
    resultados := ejecutarPrueba(5, 5, 20, t)

    fmt.Printf("\n=== Resultados Test Caso 3 (5A-5B-20C) ===\n")
    fmt.Printf("Tiempo total: %v\n", resultados.tiempoTotal)
    fmt.Printf("Aviones procesados con éxito: %d\n", resultados.avionesExito)
    fmt.Printf("Aviones no procesados: %d\n", resultados.avionesFallo)
    fmt.Printf("Tiempo promedio por avión: %v\n", resultados.tiempoPromedio)
}

// Benchmark para comparar rendimiento entre casos
func BenchmarkCasos(b *testing.B) {
    casos := []struct {
        nombre string
        numA   int
        numB   int
        numC   int
    }{
        {"CasoBase_10-10-10", 10, 10, 10},
        {"CargaAlta_20-5-5", 20, 5, 5},
        {"CargaBaja_5-5-20", 5, 5, 20},
    }

    for _, caso := range casos {
        b.Run(caso.nombre, func(b *testing.B) {
            for i := 0; i < b.N; i++ {
                ejecutarPrueba(caso.numA, caso.numB, caso.numC, nil)
            }
        })
    }
}
